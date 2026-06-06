package helper

import (
	"context"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/lingyuins/octopus/internal/conf"
	appmodel "github.com/lingyuins/octopus/internal/model"
	transmodel "github.com/lingyuins/octopus/internal/transformer/model"
	"github.com/lingyuins/octopus/internal/transformer/outbound"
	"github.com/lingyuins/octopus/internal/utils/log"
)

type GroupModelTestRequest struct {
	GroupID int `json:"group_id" binding:"required"`
}

type GroupModelDraftTestItem struct {
	ClientID  string `json:"client_id" binding:"required"`
	ChannelID int    `json:"channel_id" binding:"required"`
	ModelName string `json:"model_name" binding:"required"`
}

type GroupModelDraftTestRequest struct {
	EndpointType string                    `json:"endpoint_type" binding:"required"`
	Items        []GroupModelDraftTestItem `json:"items" binding:"required"`
}

type GroupModelTestResult struct {
	ClientID     string `json:"client_id,omitempty"`
	ItemID       int    `json:"item_id"`
	ChannelID    int    `json:"channel_id"`
	ChannelName  string `json:"channel_name"`
	ModelName    string `json:"model_name"`
	Passed       bool   `json:"passed"`
	Attempts     int    `json:"attempts"`
	StatusCode   int    `json:"status_code"`
	ResponseText string `json:"response_text,omitempty"`
	Message      string `json:"message,omitempty"`
}

type GroupModelTestSummary struct {
	Passed    bool                   `json:"passed"`
	Completed int                    `json:"completed"`
	Total     int                    `json:"total"`
	Results   []GroupModelTestResult `json:"results"`
}

type GroupModelTestProgress struct {
	ID        string                 `json:"id"`
	Passed    bool                   `json:"passed"`
	Completed int                    `json:"completed"`
	Total     int                    `json:"total"`
	Done      bool                   `json:"done"`
	Results   []GroupModelTestResult `json:"results"`
	Message   string                 `json:"message,omitempty"`
}

type groupModelTestProgressEntry struct {
	progress  GroupModelTestProgress
	expiresAt time.Time
}

var groupProbeProgress sync.Map

var groupProbeProgressTTL = 10 * time.Minute

const maxConcurrentGroupModelTests = 6

func TestGroupModels(ctx context.Context, group *appmodel.Group, channels map[int]appmodel.Channel) (*GroupModelTestSummary, error) {
	if conf.IsDevMockSuccess() {
		return buildDevMockGroupTestSummary(group)
	}
	progress := &GroupModelTestProgress{Total: len(group.Items)}
	return runGroupModelTest(ctx, group, channels, progress)
}

func StartGroupModelTest(group *appmodel.Group, channels map[int]appmodel.Channel) (*GroupModelTestProgress, error) {
	if group == nil {
		return nil, fmt.Errorf("group is nil")
	}
	if len(group.Items) == 0 {
		return nil, fmt.Errorf("group has no items")
	}
	if conf.IsDevMockSuccess() {
		summary, err := buildDevMockGroupTestSummary(group)
		if err != nil {
			return nil, err
		}
		progress := &GroupModelTestProgress{
			ID:        uuid.NewString(),
			Passed:    summary.Passed,
			Completed: summary.Completed,
			Total:     summary.Total,
			Done:      true,
			Results:   append([]GroupModelTestResult(nil), summary.Results...),
			Message:   "dev mock success",
		}
		storeGroupModelProgress(progress)
		log.Infof("dev mock group test success: group=%s total=%d", group.Name, len(group.Items))
		return progress, nil
	}

	id := uuid.NewString()
	progress := &GroupModelTestProgress{
		ID:      id,
		Total:   len(group.Items),
		Results: make([]GroupModelTestResult, 0, len(group.Items)),
	}
	storeGroupModelProgress(progress)
	log.Infof("start group test: group=%s progress_id=%s items=%d channels=%d", group.Name, id, len(group.Items), len(channels))

	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Errorf("group model test panic: group=%s progress_id=%s err=%v", group.Name, id, r)
				failed := cloneGroupModelProgress(progress)
				failed.Done = true
				failed.Passed = false
				failed.Message = fmt.Sprintf("internal error: %v", r)
				storeGroupModelProgress(&failed)
			}
		}()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		if _, err := runGroupModelTest(ctx, group, channels, progress); err != nil {
			log.Errorf("group model test failed: group=%s progress_id=%s err=%v", group.Name, id, err)
			failed := cloneGroupModelProgress(progress)
			failed.Done = true
			failed.Message = err.Error()
			storeGroupModelProgress(&failed)
			return
		}
		log.Infof("group model test completed: group=%s progress_id=%s", group.Name, id)
	}()

	cloned := cloneGroupModelProgress(progress)
	return &cloned, nil
}

func StartDraftGroupModelTest(endpointType string, items []GroupModelDraftTestItem, channels map[int]appmodel.Channel) (*GroupModelTestProgress, error) {
	log.Infof("start draft group test: endpoint_type=%s items=%d channels=%d", endpointType, len(items), len(channels))
	group := &appmodel.Group{
		Name:         "draft-group-test",
		EndpointType: appmodel.NormalizeEndpointType(endpointType),
		Items:        make([]appmodel.GroupItem, 0, len(items)),
	}

	for index, item := range items {
		log.Infof("draft group test item: index=%d channel_id=%d model=%s", index, item.ChannelID, strings.TrimSpace(item.ModelName))
		group.Items = append(group.Items, appmodel.GroupItem{
			ID:        index + 1,
			ChannelID: item.ChannelID,
			ModelName: strings.TrimSpace(item.ModelName),
			Priority:  index + 1,
			Weight:    1,
		})
	}

	id := uuid.NewString()
	progress := &GroupModelTestProgress{
		ID:      id,
		Total:   len(group.Items),
		Results: make([]GroupModelTestResult, 0, len(group.Items)),
	}
	storeGroupModelProgress(progress)

	clientIDs := make(map[int]string, len(items))
	for index, item := range items {
		clientIDs[index+1] = strings.TrimSpace(item.ClientID)
	}

	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Errorf("draft group model test panic: progress_id=%s err=%v", id, r)
				failed := cloneGroupModelProgress(progress)
				failed.Done = true
				failed.Passed = false
				failed.Message = fmt.Sprintf("internal error: %v", r)
				storeGroupModelProgress(&failed)
			}
		}()

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		summary, err := runGroupModelTest(ctx, group, channels, progress)
		if err != nil {
			log.Errorf("draft group model test failed: progress_id=%s err=%v", id, err)
			failed := cloneGroupModelProgress(progress)
			failed.Done = true
			failed.Message = err.Error()
			for i := range failed.Results {
				if clientID, ok := clientIDs[failed.Results[i].ItemID]; ok {
					failed.Results[i].ClientID = clientID
				}
			}
			storeGroupModelProgress(&failed)
			return
		}

		final := cloneGroupModelProgress(progress)
		final.Passed = summary.Passed
		final.Completed = summary.Completed
		final.Total = summary.Total
		final.Done = true
		final.Results = append([]GroupModelTestResult(nil), summary.Results...)
		for i := range final.Results {
			if clientID, ok := clientIDs[final.Results[i].ItemID]; ok {
				final.Results[i].ClientID = clientID
			}
		}
		storeGroupModelProgress(&final)
	}()

	cloned := cloneGroupModelProgress(progress)
	return &cloned, nil
}

func GetGroupModelTestProgress(id string) (*GroupModelTestProgress, bool) {
	cleanupExpiredGroupModelProgress(time.Now())

	value, ok := groupProbeProgress.Load(id)
	if !ok {
		return nil, false
	}

	entry, ok := value.(groupModelTestProgressEntry)
	if !ok {
		return nil, false
	}

	cloned := cloneGroupModelProgress(&entry.progress)
	return &cloned, true
}

func runGroupModelTest(ctx context.Context, group *appmodel.Group, channels map[int]appmodel.Channel, progress *GroupModelTestProgress) (*GroupModelTestSummary, error) {
	if group == nil {
		return nil, fmt.Errorf("group is nil")
	}
	if len(group.Items) == 0 {
		return nil, fmt.Errorf("group has no items")
	}

	summary := &GroupModelTestSummary{Total: len(group.Items), Results: make([]GroupModelTestResult, 0, len(group.Items))}
	workerCount := int(math.Min(float64(len(group.Items)), maxConcurrentGroupModelTests))
	if workerCount < 1 {
		workerCount = 1
	}

	type indexedResult struct {
		index  int
		result GroupModelTestResult
	}

	resultsByIndex := make([]GroupModelTestResult, len(group.Items))
	jobs := make(chan int)
	results := make(chan indexedResult, len(group.Items))
	var wg sync.WaitGroup

	for workerIndex := 0; workerIndex < workerCount; workerIndex++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for index := range jobs {
				results <- indexedResult{index: index, result: testGroupModelItem(ctx, group.EndpointType, group.Items[index], channels)}
			}
		}()
	}

	go func() {
		for index := range group.Items {
			if ctx.Err() != nil {
				break
			}
			jobs <- index
		}
		close(jobs)
		wg.Wait()
		close(results)
	}()

	for indexed := range results {
		resultsByIndex[indexed.index] = indexed.result
	}

	for _, result := range resultsByIndex {
		appendGroupTestResult(summary, progress, result)
	}

	if progress != nil {
		finalProgress := cloneGroupModelProgress(progress)
		finalProgress.Done = true
		finalProgress.Passed = summary.Passed
		storeGroupModelProgress(&finalProgress)
	}

	return summary, nil
}

func testGroupModelItem(ctx context.Context, endpointType string, item appmodel.GroupItem, channels map[int]appmodel.Channel) GroupModelTestResult {
	result := GroupModelTestResult{
		ItemID:    item.ID,
		ChannelID: item.ChannelID,
		ModelName: item.ModelName,
		Attempts:  3,
	}

	channel, ok := channels[item.ChannelID]
	if !ok {
		result.Message = "channel not found"
		return result
	}
	result.ChannelName = channel.Name
	if !channel.Enabled {
		result.Message = "channel disabled"
		return result
	}

	usedKey := channel.GetChannelKey()
	if strings.TrimSpace(usedKey.ChannelKey) == "" {
		result.Message = "no available key"
		return result
	}

	outAdapter := outbound.Get(channel.Type)
	if outAdapter == nil {
		result.Message = fmt.Sprintf("unsupported channel type: %d", channel.Type)
		return result
	}

	if err := validateGroupProbeChannelType(endpointType, channel.Type); err != nil {
		result.Message = err.Error()
		return result
	}

	for attempt := 1; attempt <= 3; attempt++ {
		if attempt > 1 && ctx.Err() != nil {
			result.Message = ctx.Err().Error()
			break
		}
		statusCode, responseText, err := sendGroupProbeRequest(ctx, outAdapter, &channel, usedKey.ChannelKey, endpointType, item.ModelName)
		result.StatusCode = statusCode
		result.ResponseText = responseText
		if err == nil {
			result.Passed = true
			result.Attempts = attempt
			result.Message = "ok"
			break
		}
		result.Attempts = attempt
		result.Message = err.Error()
	}

	return result
}

func appendGroupTestResult(summary *GroupModelTestSummary, progress *GroupModelTestProgress, result GroupModelTestResult) {
	log.Infof("group test result: item_id=%d channel_id=%d model=%s passed=%t status=%d message=%s", result.ItemID, result.ChannelID, result.ModelName, result.Passed, result.StatusCode, result.Message)
	summary.Results = append(summary.Results, result)
	summary.Completed = len(summary.Results)
	if result.Passed {
		summary.Passed = true
	}

	if progress == nil {
		return
	}

	next := cloneGroupModelProgress(progress)
	next.Results = append(next.Results, result)
	next.Completed = len(next.Results)
	next.Passed = summary.Passed
	storeGroupModelProgress(&next)
}

func storeGroupModelProgress(progress *GroupModelTestProgress) {
	storeGroupModelProgressAt(progress, time.Now())
}

func storeGroupModelProgressAt(progress *GroupModelTestProgress, now time.Time) {
	if progress == nil || progress.ID == "" {
		return
	}

	cleanupExpiredGroupModelProgress(now)
	groupProbeProgress.Store(progress.ID, groupModelTestProgressEntry{
		progress:  cloneGroupModelProgress(progress),
		expiresAt: now.Add(groupProbeProgressTTL),
	})
}

func cleanupExpiredGroupModelProgress(now time.Time) {
	groupProbeProgress.Range(func(key, value any) bool {
		entry, ok := value.(groupModelTestProgressEntry)
		if !ok || (!entry.expiresAt.IsZero() && !now.Before(entry.expiresAt)) {
			groupProbeProgress.Delete(key)
		}
		return true
	})
}

func cloneGroupModelProgress(progress *GroupModelTestProgress) GroupModelTestProgress {
	if progress == nil {
		return GroupModelTestProgress{}
	}

	cloned := *progress
	if progress.Results != nil {
		cloned.Results = append([]GroupModelTestResult(nil), progress.Results...)
	}
	return cloned
}

func sendGroupProbeRequest(ctx context.Context, outAdapter transmodel.Outbound, channel *appmodel.Channel, key, endpointType, modelName string) (int, string, error) {
	if channel == nil {
		return 0, "", fmt.Errorf("channel is nil")
	}

	httpClient, err := ChannelHttpClient(channel)
	if err != nil {
		return 0, "", err
	}

	probeRequest, err := buildGroupProbeRequest(endpointType, modelName)
	if err != nil {
		return 0, "", err
	}

	req, err := outAdapter.TransformRequest(ctx, probeRequest, channel.GetNormalizedBaseUrl(), key)
	if err != nil {
		return 0, "", err
	}

	for _, header := range channel.CustomHeader {
		if strings.TrimSpace(header.HeaderKey) != "" {
			req.Header.Set(header.HeaderKey, header.HeaderValue)
		}
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return 0, "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	bodyText := strings.TrimSpace(string(body))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if bodyText == "" {
			bodyText = resp.Status
		}
		return resp.StatusCode, bodyText, fmt.Errorf("upstream error: %d", resp.StatusCode)
	}

	if _, err := outAdapter.TransformResponse(ctx, &http.Response{
		Status:        resp.Status,
		StatusCode:    resp.StatusCode,
		Header:        resp.Header.Clone(),
		Body:          io.NopCloser(strings.NewReader(bodyText)),
		ContentLength: int64(len(bodyText)),
	}); err != nil {
		return resp.StatusCode, bodyText, err
	}

	return resp.StatusCode, bodyText, nil
}

func buildGroupProbeRequest(endpointType, modelName string) (*transmodel.InternalLLMRequest, error) {
	stream := false
	normalizedEndpointType := normalizeGroupProbeEndpointType(endpointType)

	switch {
	case normalizedEndpointType == appmodel.EndpointTypeEmbeddings:
		return &transmodel.InternalLLMRequest{
			Model:          modelName,
			EmbeddingInput: &transmodel.EmbeddingInput{Single: stringPtr("hi")},
		}, nil
	case normalizedEndpointType == appmodel.EndpointTypeAll || appmodel.IsConversationEndpointType(normalizedEndpointType):
		return &transmodel.InternalLLMRequest{
			Model: modelName,
			Messages: []transmodel.Message{{
				Role: "user",
				Content: transmodel.MessageContent{
					Content: stringPtr("hi"),
				},
			}},
			Stream: &stream,
		}, nil
	default:
		return nil, fmt.Errorf("group probe does not support endpoint type: %s", normalizedEndpointType)
	}
}

func validateGroupProbeChannelType(endpointType string, channelType outbound.OutboundType) error {
	normalizedEndpointType := normalizeGroupProbeEndpointType(endpointType)

	switch normalizedEndpointType {
	case appmodel.EndpointTypeEmbeddings:
		if !outbound.IsEmbeddingChannelType(channelType) {
			return fmt.Errorf("channel type %d does not support endpoint type %s", channelType, appmodel.EndpointTypeEmbeddings)
		}
		return nil
	case appmodel.EndpointTypeAll:
		if !outbound.IsChatChannelType(channelType) {
			return fmt.Errorf("channel type %d does not support endpoint type %s", channelType, appmodel.EndpointTypeAll)
		}
		return nil
	default:
		if appmodel.IsConversationEndpointType(normalizedEndpointType) {
			if !outbound.IsChatChannelType(channelType) {
				return fmt.Errorf("channel type %d does not support endpoint type %s", channelType, normalizedEndpointType)
			}
			return nil
		}
		return fmt.Errorf("group probe does not support endpoint type: %s", normalizedEndpointType)
	}

	return nil
}

func normalizeGroupProbeEndpointType(endpointType string) string {
	return appmodel.NormalizeEndpointType(endpointType)
}

func stringPtr(value string) *string {
	return &value
}

func buildDevMockGroupTestSummary(group *appmodel.Group) (*GroupModelTestSummary, error) {
	if group == nil {
		return nil, fmt.Errorf("group is nil")
	}
	if len(group.Items) == 0 {
		return nil, fmt.Errorf("group has no items")
	}

	results := make([]GroupModelTestResult, 0, len(group.Items))
	for _, item := range group.Items {
		results = append(results, GroupModelTestResult{
			ItemID:       item.ID,
			ChannelID:    item.ChannelID,
			ChannelName:  "dev-mock-channel",
			ModelName:    item.ModelName,
			Passed:       true,
			Attempts:     1,
			StatusCode:   http.StatusOK,
			ResponseText: devMockText,
			Message:      "ok",
		})
	}
	return &GroupModelTestSummary{
		Passed:    true,
		Completed: len(results),
		Total:     len(results),
		Results:   results,
	}, nil
}
