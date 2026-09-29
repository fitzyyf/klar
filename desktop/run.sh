#!/bin/sh
# 重新打包并启动客户端。说「启动」就跑这个：先杀旧实例，make app，再开窗口。
set -e
cd "$(dirname "$0")"

pkill -f "Agent会话Review.app/Contents/MacOS/klar" 2>/dev/null || true

if ! make app >/tmp/klar-make.log 2>&1; then
  tail -30 /tmp/klar-make.log
  exit 1
fi

if ! open "Agent会话Review.app"; then
  ./Agent会话Review.app/Contents/MacOS/klar >/tmp/klar-gui.log 2>&1 &
fi

sleep 3
if ! pgrep -f "Agent会话Review.app/Contents/MacOS/klar" >/dev/null; then
  echo "open 启动没活下来，改直接跑"
  ./Agent会话Review.app/Contents/MacOS/klar >/tmp/klar-gui.log 2>&1 &
  sleep 3
fi

pgrep -lf "Agent会话Review.app/Contents/MacOS/klar" || { echo "没起来，看 /tmp/klar-gui.log 和 /tmp/klar-make.log"; exit 1; }
echo "起来了"
