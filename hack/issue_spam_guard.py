#!/usr/bin/env python3
"""issue/PR 提交时的垃圾账号拦截。供 issue-spam-guard 工作流调用。"""
import argparse
import json
import os
import re
import sys
import urllib.error
import urllib.request
from datetime import datetime, timezone

API = "https://api.github.com"
START_MARK = "<!-- BLACKLIST:START -->"
END_MARK = "<!-- BLACKLIST:END -->"
USERNAME_RE = re.compile(r"^[A-Za-z0-9][A-Za-z0-9-]{0,38}$")


def log(msg):
    print(f"[{datetime.now(timezone.utc).strftime('%H:%M:%S')}] {msg}", flush=True)


def http_request(method, url, token, payload=None, accept="application/vnd.github+json"):
    """返回 (status, parsed_or_raw)。errors 不抛出，交由调用方判断。"""
    headers = {
        "Authorization": f"Bearer {token}",
        "Accept": accept,
        "X-GitHub-Api-Version": "2022-11-28",
        "User-Agent": "pixiu-issue-spam-guard",
    }
    data = None
    if payload is not None:
        data = json.dumps(payload).encode("utf-8")
        headers["Content-Type"] = "application/json"
    req = urllib.request.Request(url, data=data, headers=headers, method=method)
    try:
        with urllib.request.urlopen(req, timeout=25) as resp:
            raw = resp.read().decode("utf-8", "replace")
            if accept == "application/vnd.github.raw":
                return resp.status, raw
            return resp.status, (json.loads(raw) if raw else None)
    except urllib.error.HTTPError as e:
        raw = e.read().decode("utf-8", "replace")
        try:
            parsed = json.loads(raw) if raw else None
        except Exception:
            parsed = raw
        return e.code, parsed
    except Exception as e:
        log(f"HTTP 请求异常 ({method}): {type(e).__name__}")
        return 0, None


def fetch_blacklist(token, bl_repo, bl_path):
    """读取黑名单，返回 (names:set, error:str|None)。"""
    if not token:
        return set(), "未提供 token，无法读取私有黑名单仓库"
    url = f"{API}/repos/{bl_repo}/contents/{bl_path}"
    status, data = http_request("GET", url, token, accept="application/vnd.github.raw")
    if status != 200 or not isinstance(data, str):
        return set(), f"读取黑名单失败 (HTTP {status})"
    return parse_blacklist(data), None


def parse_blacklist(text):
    """解析 <!-- BLACKLIST:START --> ... <!-- BLACKLIST:END --> 块内的用户名。"""
    names = set()
    in_block = False
    for line in text.splitlines():
        s = line.strip()
        if s == START_MARK:
            in_block = True
            continue
        if s == END_MARK:
            in_block = False
            continue
        if not in_block:
            continue
        # 去掉行尾注释
        s = s.split("#", 1)[0].strip()
        # 允许 "- username" 列表写法
        if s.startswith("- "):
            s = s[2:].strip()
        if not s:
            continue
        if USERNAME_RE.match(s):
            names.add(s.lower())
        else:
            log("忽略非法用户名行")
    return names


def load_event(path):
    """读取事件 JSON。"""
    with open(path, "r", encoding="utf-8") as fh:
        event = json.load(fh)
    repo = (event.get("repository") or {}).get("full_name", "")
    is_pr = "pull_request" in event
    obj = event.get("pull_request") or event.get("issue") or {}
    return {
        "number": obj.get("number"),
        "author": (obj.get("user") or {}).get("login", ""),
        "title": obj.get("title", "") or "",
        "body": obj.get("body", "") or "",
        "repo": repo,
        "is_pr": is_pr,
    }


def delete_issue(token, repo, number):
    status, _ = http_request("DELETE", f"{API}/repos/{repo}/issues/{number}", token)
    # 204=删除成功；404=已不存在（视为达成）
    return status in (200, 204, 404), status


def close_pr(token, repo, number):
    status, _ = http_request("PATCH", f"{API}/repos/{repo}/pulls/{number}",
                             token, payload={"state": "closed"})
    # 200=关闭成功；404=已不存在（视为达成）
    return status in (200, 404), status


def main():
    p = argparse.ArgumentParser()
    p.add_argument("--issue-number", type=int)
    p.add_argument("--author")
    p.add_argument("--title", default="")
    p.add_argument("--body", default="")
    p.add_argument("--is-pr", action="store_true", help="目标是 PR")
    p.add_argument("--from-event", default=os.environ.get("GITHUB_EVENT_PATH", ""),
                   help="从事件 JSON 读取")
    p.add_argument("--repo", default=os.environ.get("GITHUB_REPOSITORY", ""))
    p.add_argument("--bl-repo", default=os.environ.get("BLACKLIST_REPO", ""),
                   help="黑名单文档所在仓库（必填）")
    p.add_argument("--bl-path", default=os.environ.get("BLACKLIST_PATH", ""),
                   help="黑名单文档路径（必填）")
    p.add_argument("--token", default=os.environ.get("BLACK_TOKEN", os.environ.get("GITHUB_TOKEN", "")))
    p.add_argument("--apply", action="store_true", help="真正执行；默认仅判定")
    args = p.parse_args()

    if not args.bl_repo or not args.bl_path:
        log("缺少黑名单源配置（BLACKLIST_REPO / BLACKLIST_PATH）")
        sys.exit(2)

    if args.from_event and os.path.exists(args.from_event):
        ev = load_event(args.from_event)
        number, author, repo, is_pr = ev["number"], ev["author"], ev["repo"] or args.repo, ev["is_pr"]
    else:
        number, author, repo, is_pr = args.issue_number, args.author, args.repo, args.is_pr

    if not repo or not number or not author:
        log("缺少 repo / number / author")
        sys.exit(2)

    kind = "PR" if is_pr else "issue"
    log(f"检测 {kind} {repo}#{number} author={author}")

    names, err = fetch_blacklist(args.token, args.bl_repo, args.bl_path)
    if err:
        log("黑名单读取失败，放行")
        print("decision=allow")
        return
    if author.lower() in names:
        blocked, reason = True, "作者命中黑名单"
    else:
        blocked, reason = False, ""

    if not blocked:
        log("结论：未命中黑名单，放行")
        print("decision=allow")
        return

    log(f"结论：命中拦截 —— {reason}")
    print("decision=block")
    print(f"reason={reason}")

    if not args.apply:
        log("[dry-run] 未执行删除/关闭（加 --apply 生效）")
        return

    if not args.token:
        log("未提供 token，无法删除/关闭（GITHUB_TOKEN 默认无权删 issue）")
        sys.exit(3)

    if is_pr:
        ok, code = close_pr(args.token, repo, number)
        log(f"关闭 PR #{number}: {'成功' if ok else '失败'} (HTTP {code})")
    else:
        ok, code = delete_issue(args.token, repo, number)
        log(f"删除 issue #{number}: {'成功' if ok else '失败'} (HTTP {code})")

    if not ok:
        sys.exit(4)


if __name__ == "__main__":
    main()

