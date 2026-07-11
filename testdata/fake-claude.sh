#!/usr/bin/env bash
# 模拟 claude --print --output-format stream-json:
# 读取一行 stdin(JSON), 输出 thinking → 两个 token → done 的 stream-json 行。
while read -r _line; do
  printf '{"type":"thinking"}\n'
  printf '{"type":"token","content":"hello "}\n'
  printf '{"type":"token","content":"world"}\n'
  printf '{"type":"done"}\n'
done
