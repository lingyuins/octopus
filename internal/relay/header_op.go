package relay

import (
	"net/http"
	"strings"

	"github.com/bestruirui/octopus/internal/model"
)

// applyHeaderOps 按顺序对 outbound request header 执行 op 数组。
// 单个 op 失败不中断（继续下一个），错误在日志层面被忽略。
// 大小写：name/from/to 都会被 canonical 化以匹配 http.Header。
func applyHeaderOps(req *http.Request, ops []model.HeaderOp) {
	if len(ops) == 0 {
		return
	}
	for _, op := range ops {
		switch strings.ToLower(op.Op) {
		case "set":
			name := http.CanonicalHeaderKey(op.Name)
			if name == "" {
				continue
			}
			req.Header.Set(name, op.Value)
		case "add":
			name := http.CanonicalHeaderKey(op.Name)
			if name == "" {
				continue
			}
			req.Header.Add(name, op.Value)
		case "delete":
			name := http.CanonicalHeaderKey(op.Name)
			if name == "" {
				continue
			}
			req.Header.Del(name)
		case "rename":
			from := http.CanonicalHeaderKey(op.Name)
			to := http.CanonicalHeaderKey(op.To)
			if from == "" || to == "" || from == to {
				continue
			}
			values := req.Header.Values(from)
			if len(values) == 0 {
				continue
			}
			req.Header.Del(from)
			for _, v := range values {
				req.Header.Add(to, v)
			}
		case "copy":
			to := http.CanonicalHeaderKey(op.Name)
			from := http.CanonicalHeaderKey(op.From)
			if to == "" || from == "" {
				continue
			}
			if v := req.Header.Get(from); v != "" {
				req.Header.Set(to, v)
			}
		}
	}
}
