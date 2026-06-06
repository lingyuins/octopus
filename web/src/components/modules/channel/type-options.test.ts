import assert from "node:assert/strict";
import test from "node:test";

import { CHANNEL_TYPE_OPTIONS } from "./type-options.ts";

const OpenAIChat = 0;
const OpenAIResponse = 1;

test("channel type options merge OpenAI chat and response into one OpenAI option", () => {
  const openaiOptions = CHANNEL_TYPE_OPTIONS.filter(
    (option) => option.value === OpenAIChat || option.value === OpenAIResponse,
  );

  assert.deepEqual(openaiOptions, [
    { value: OpenAIChat, labelKey: "typeOpenAI" },
  ]);
});
