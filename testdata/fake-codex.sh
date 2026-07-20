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
if [ "$prompt" = "HUGE_STDERR" ]; then
  dd if=/dev/zero bs=1048576 count=1 2>/dev/null | tr '\000' 's' >&2
  exit 9
fi
if [ "$prompt" = "HUGE_STDOUT" ]; then
  dd if=/dev/zero bs=1048576 count=5 2>/dev/null | tr '\000' 'x'
  printf '\n'
  exit 0
fi

printf '%s\n' '{"type":"thread.started","thread_id":"thread-fake"}'
printf '%s\n' '{"type":"turn.started"}'
printf '%s\n' '{"type":"item.completed","item":{"id":"item-1","type":"agent_message","text":"FAKE_CODEX_OK"}}'
printf '%s\n' '{"type":"turn.completed","usage":{"input_tokens":1,"output_tokens":1}}'
