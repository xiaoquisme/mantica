#!/usr/bin/env python3
"""
Session JSONL → task_message 提取器

从 Hermes session JSONL 文件中提取 tool_use/tool_result 对，
转换为 Multica task_message 格式。

用法:
  python3 extract_session.py <session_file> [--task-id <uuid>]

输出: JSON 数组，每个元素是一条 task_message 记录
"""

import json
import sys
import os
import glob
import argparse
from pathlib import Path
from typing import Optional


def find_session_file(session_id: str, hermes_home: str = None) -> Optional[str]:
    """根据 session_id 查找对应的 JSONL 文件"""
    if hermes_home is None:
        hermes_home = os.path.expanduser("~/.hermes")
    
    sessions_dir = os.path.join(hermes_home, "sessions")
    if not os.path.isdir(sessions_dir):
        return None
    
    # session 文件名格式: {timestamp}_{8位hex}.jsonl
    # session_id 格式: {timestamp}_{8位hex}
    # 文件名就是 session_id + ".jsonl"
    direct = os.path.join(sessions_dir, f"{session_id}.jsonl")
    if os.path.isfile(direct):
        return direct
    
    # 模糊匹配（session_id 可能是完整文件名的一部分）
    pattern = os.path.join(sessions_dir, f"*{session_id}*.jsonl")
    matches = glob.glob(pattern)
    if matches:
        # 返回最新的
        return sorted(matches, key=os.path.getmtime, reverse=True)[0]
    
    return None


def parse_session_messages(filepath: str) -> list[dict]:
    """
    解析 session JSONL，提取 tool_use + tool_result 对。
    
    返回结构化的消息列表，每条包含:
      - type: "tool_use" | "tool_result" | "thinking" | "text"
      - tool: 工具名称
      - input: 工具输入 (dict)
      - output: 工具输出 (str)
      - content: 文本内容
      - timestamp: 时间戳
    """
    messages = []
    pending_tool_uses = {}  # call_id -> tool_use info
    
    with open(filepath, "r") as f:
        for line_num, line in enumerate(f, 1):
            line = line.strip()
            if not line:
                continue
            try:
                entry = json.loads(line)
            except json.JSONDecodeError:
                continue
            
            role = entry.get("role", "")
            timestamp = entry.get("timestamp", "")
            
            if role == "assistant":
                # 提取 tool_calls
                tool_calls = entry.get("tool_calls", [])
                for tc in tool_calls:
                    call_id = tc.get("call_id", tc.get("id", ""))
                    func = tc.get("function", {})
                    tool_name = func.get("name", "unknown")
                    raw_args = func.get("arguments", "{}")
                    
                    # 解析 arguments
                    if isinstance(raw_args, str):
                        try:
                            tool_input = json.loads(raw_args)
                        except json.JSONDecodeError:
                            tool_input = {"_raw": raw_args}
                    elif isinstance(raw_args, dict):
                        tool_input = raw_args
                    else:
                        tool_input = {"_raw": str(raw_args)}
                    
                    msg = {
                        "type": "tool_use",
                        "tool": tool_name,
                        "input": tool_input,
                        "call_id": call_id,
                        "timestamp": timestamp,
                    }
                    messages.append(msg)
                    
                    if call_id:
                        pending_tool_uses[call_id] = msg
                
                # 提取 reasoning
                reasoning = entry.get("reasoning_content", "")
                if reasoning:
                    messages.append({
                        "type": "thinking",
                        "content": reasoning,
                        "timestamp": timestamp,
                    })
                
                # 提取 text content
                content = entry.get("content", "")
                if content and not tool_calls:
                    messages.append({
                        "type": "text",
                        "content": content,
                        "timestamp": timestamp,
                    })
            
            elif role == "tool":
                call_id = entry.get("tool_call_id", "")
                tool_name = entry.get("name", "")
                content = entry.get("content", "")
                
                # 尝试截断过长的 output
                if len(content) > 8192:
                    content = content[:8192] + "...[truncated]"
                
                msg = {
                    "type": "tool_result",
                    "tool": tool_name,
                    "output": content,
                    "call_id": call_id,
                    "timestamp": timestamp,
                }
                
                # 关联到对应的 tool_use
                if call_id and call_id in pending_tool_uses:
                    use_msg = pending_tool_uses[call_id]
                    msg["tool"] = use_msg.get("tool", tool_name)
                    msg["input"] = use_msg.get("input", {})
                
                messages.append(msg)
    
    return messages


def to_task_messages(messages: list[dict], task_id: str) -> list[dict]:
    """
    转换为 Multica task_message 格式 (适配 CreateTaskMessage SQL)
    
    CREATE TABLE task_message (
        id UUID, task_id UUID, seq INT, type TEXT,
        tool TEXT, content TEXT, input JSONB, output TEXT
    )
    """
    task_messages = []
    seq = 1
    
    for msg in messages:
        msg_type = msg.get("type", "")
        
        if msg_type == "tool_use":
            task_messages.append({
                "task_id": task_id,
                "seq": seq,
                "type": "tool_use",
                "tool": msg.get("tool", ""),
                "content": None,
                "input": json.dumps(msg.get("input", {}), ensure_ascii=False),
                "output": None,
                "timestamp": msg.get("timestamp", ""),
            })
            seq += 1
        
        elif msg_type == "tool_result":
            task_messages.append({
                "task_id": task_id,
                "seq": seq,
                "type": "tool_result",
                "tool": msg.get("tool", ""),
                "content": None,
                "input": None,
                "output": msg.get("output", ""),
                "timestamp": msg.get("timestamp", ""),
            })
            seq += 1
        
        elif msg_type == "thinking":
            task_messages.append({
                "task_id": task_id,
                "seq": seq,
                "type": "thinking",
                "tool": None,
                "content": msg.get("content", ""),
                "input": None,
                "output": None,
                "timestamp": msg.get("timestamp", ""),
            })
            seq += 1
        
        elif msg_type == "text":
            task_messages.append({
                "task_id": task_id,
                "seq": seq,
                "type": "text",
                "tool": None,
                "content": msg.get("content", ""),
                "input": None,
                "output": None,
                "timestamp": msg.get("timestamp", ""),
            })
            seq += 1
    
    return task_messages


def compute_stats(messages: list[dict]) -> dict:
    """计算执行统计"""
    tool_uses = [m for m in messages if m["type"] == "tool_use"]
    tool_results = [m for m in messages if m["type"] == "tool_result"]
    errors = [m for m in messages if m["type"] == "tool_result" and _is_error(m)]
    
    # 工具使用统计
    tool_counts = {}
    for m in tool_uses:
        name = m.get("tool", "unknown")
        tool_counts[name] = tool_counts.get(name, 0) + 1
    
    return {
        "total_steps": len(messages),
        "tool_use_count": len(tool_uses),
        "tool_result_count": len(tool_results),
        "error_count": len(errors),
        "tool_counts": tool_counts,
        "unique_tools": list(tool_counts.keys()),
    }


def _is_error(msg: dict) -> bool:
    """判断 tool_result 是否是错误"""
    output = msg.get("output", "").lower()
    error_indicators = ["error", "traceback", "exception", "failed", "permission denied", "not found"]
    return any(ind in output for ind in error_indicators)


def main():
    parser = argparse.ArgumentParser(description="Extract tool messages from Hermes session JSONL")
    parser.add_argument("session_file", help="Path to session JSONL file or session_id")
    parser.add_argument("--task-id", default="00000000-0000-0000-0000-000000000000", help="Task UUID")
    parser.add_argument("--stats", action="store_true", help="Output stats instead of messages")
    parser.add_argument("--hermes-home", default=None, help="Hermes home directory")
    args = parser.parse_args()
    
    # 如果不是文件路径，尝试作为 session_id 查找
    filepath = args.session_file
    if not os.path.isfile(filepath):
        filepath = find_session_file(args.session_file, args.hermes_home)
        if not filepath:
            print(f"Error: session file not found for '{args.session_file}'", file=sys.stderr)
            sys.exit(1)
    
    messages = parse_session_messages(filepath)
    
    if args.stats:
        stats = compute_stats(messages)
        print(json.dumps(stats, indent=2, ensure_ascii=False))
    else:
        task_messages = to_task_messages(messages, args.task_id)
        print(json.dumps(task_messages, indent=2, ensure_ascii=False))


if __name__ == "__main__":
    main()
