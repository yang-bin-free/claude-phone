#!/bin/sh
prompt=""
for value in "$@"; do
  prompt="$value"
done

if [ "$prompt" = "SLOW" ]; then
  sleep 0.3
fi
if [ "$prompt" = "FAIL" ]; then
  echo "simulated Codex failure" >&2
  exit 7
fi

printf '%s\n' '{"type":"thread.started","thread_id":"thread-fake"}'
printf '%s\n' '{"type":"turn.started"}'
printf '%s\n' '{"type":"item.completed","item":{"id":"item-1","type":"agent_message","text":"FAKE_CODEX_OK"}}'
printf '%s\n' '{"type":"turn.completed","usage":{"input_tokens":1,"output_tokens":1}}'
