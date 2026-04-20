#!/usr/bin/env python3
"""Generate user-readable Chinese GitHub release notes from git history."""

from __future__ import annotations

import argparse
import json
import re
import subprocess
import sys
from dataclasses import dataclass
from pathlib import Path
from typing import Optional

REPO_ROOT = Path(__file__).resolve().parents[1]
MAPPING_PATH = REPO_ROOT / ".github" / "release-note-mapping.json"
FIELD_SEP = "\x1f"
RECORD_SEP = "\x1e"


@dataclass
class CommitEntry:
    sha: str
    subject: str
    body: str
    commit_type: str
    summary: str
    category: str


def run_git(*args: str, check: bool = True) -> str:
    result = subprocess.run(
        ["git", *args],
        cwd=REPO_ROOT,
        check=False,
        capture_output=True,
        text=True,
    )
    if check and result.returncode != 0:
        raise RuntimeError(result.stderr.strip() or "git command failed")
    return result.stdout


def load_mapping() -> dict:
    with MAPPING_PATH.open("r", encoding="utf-8") as fh:
        return json.load(fh)


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Generate Chinese release notes")
    parser.add_argument("--current-tag", required=True, help="Current release tag or ref")
    parser.add_argument("--output", required=True, help="Output markdown file path")
    return parser.parse_args()


def find_previous_tag(current_tag: str) -> Optional[str]:
    try:
        return run_git("describe", "--tags", "--abbrev=0", f"{current_tag}^").strip()
    except RuntimeError:
        return None


def release_range(current_tag: str, previous_tag: Optional[str]) -> str:
    return f"{previous_tag}..{current_tag}" if previous_tag else current_tag


def parse_commits(log_output: str) -> list[tuple[str, str, str]]:
    commits: list[tuple[str, str, str]] = []
    for record in log_output.split(RECORD_SEP):
        if not record.strip():
            continue
        parts = record.split(FIELD_SEP)
        if len(parts) != 3:
            continue
        sha, subject, body = parts
        commits.append((sha.strip(), subject.strip(), body.strip()))
    return commits


def commit_type(subject: str) -> str:
    match = re.match(r"^(?P<type>[a-zA-Z]+)(?:\([^)]+\))?!?:\s+(?P<rest>.+)$", subject)
    return match.group("type").lower() if match else "other"


def strip_conventional_prefix(subject: str) -> str:
    match = re.match(r"^[a-zA-Z]+(?:\([^)]+\))?!?:\s+(?P<rest>.+)$", subject)
    return match.group("rest").strip() if match else subject.strip()


def extract_trailer(body: str, key: str) -> Optional[str]:
    pattern = re.compile(rf"^{re.escape(key)}:\s*(.+)$", re.IGNORECASE | re.MULTILINE)
    match = pattern.search(body)
    return match.group(1).strip() if match else None


def normalize_phrase(text: str) -> str:
    text = text.replace("_", " ").replace("-", " ")
    text = re.sub(r"\s+", " ", text).strip()
    return text


def apply_phrase_map(text: str, phrase_map: dict[str, str]) -> str:
    result = f" {normalize_phrase(text)} "
    for source in sorted(phrase_map, key=len, reverse=True):
        replacement = phrase_map[source]
        pattern = re.compile(rf"(?i)(?<![A-Za-z]){re.escape(source)}(?![A-Za-z])")
        result = pattern.sub(replacement, result)
    result = re.sub(r"\s+", " ", result).strip(" ，。")
    return result


def classify_category(commit_kind: str, mapping: dict, trailer_category: Optional[str]) -> str:
    if trailer_category:
        return trailer_category
    return mapping["category_map"].get(commit_kind, mapping["default_category"])


def mapped_summary(subject: str, body: str, commit_kind: str, mapping: dict) -> tuple[str, str]:
    trailer_summary = extract_trailer(body, "Release-note-zh") or extract_trailer(body, "Release-note-cn")
    trailer_category = extract_trailer(body, "Release-note-category")
    if trailer_summary:
        return trailer_summary, classify_category(commit_kind, mapping, trailer_category)

    stripped = strip_conventional_prefix(subject)
    normalized = normalize_phrase(stripped).lower()

    exact = mapping.get("exact_map", {})
    if normalized in exact:
        item = exact[normalized]
        return item["summary"], item.get("category") or classify_category(commit_kind, mapping, trailer_category)

    for rule in mapping.get("summary_rules", []):
        if re.search(rule["pattern"], stripped, flags=re.IGNORECASE):
            return rule["summary"], rule.get("category") or classify_category(commit_kind, mapping, trailer_category)

    translated = apply_phrase_map(stripped, mapping.get("phrase_map", {}))
    translated = translated[0].upper() + translated[1:] if translated else stripped

    if not re.search(r"[\u4e00-\u9fff]", translated):
        verb_prefix = mapping.get("type_prefix_map", {}).get(commit_kind, mapping.get("fallback_prefix", "更新"))
        translated = f"{verb_prefix}{translated}"

    return translated, classify_category(commit_kind, mapping, trailer_category)


def collect_commits(ref_range: str, mapping: dict) -> list[CommitEntry]:
    raw = run_git("log", "--reverse", "--format=%H%x1f%s%x1f%b%x1e", ref_range)
    entries: list[CommitEntry] = []
    for sha, subject, body in parse_commits(raw):
        kind = commit_type(subject)
        summary, category = mapped_summary(subject, body, kind, mapping)
        entries.append(
            CommitEntry(
                sha=sha,
                subject=subject,
                body=body,
                commit_type=kind,
                summary=summary,
                category=category,
            )
        )
    return entries


def deduplicate_summaries(commits: list[CommitEntry]) -> list[CommitEntry]:
    seen: set[tuple[str, str]] = set()
    result: list[CommitEntry] = []
    for commit in commits:
        key = (commit.category, commit.summary)
        if key in seen:
            continue
        seen.add(key)
        result.append(commit)
    return result


def build_markdown(current_tag: str, previous_tag: Optional[str], commits: list[CommitEntry], mapping: dict, raw_commit_count: int) -> str:
    lines = [f"# {current_tag} 更新说明", ""]

    if commits:
        lines.append("## 更新摘要")
        for commit in commits:
            lines.append(f"- {commit.summary}")
        lines.append("")

        categories = mapping.get("category_order", [])
        for category in categories:
            category_items = [item for item in commits if item.category == category]
            if not category_items:
                continue
            lines.append(f"## {category}")
            for item in category_items:
                lines.append(f"- {item.summary}")
            lines.append("")
    else:
        lines.extend(["## 更新摘要", "- 本次版本未检测到可归纳的提交记录。", ""])

    lines.append("## 生成信息")
    if previous_tag:
        lines.append(f"- 提交范围：`{previous_tag}..{current_tag}`")
    else:
        lines.append("- 提交范围：首个自动发布版本（从仓库起点统计）")
    lines.append(f"- 提交数量：{raw_commit_count}")
    lines.append("- 摘要策略：优先使用 `Release-note-zh` trailer，其次命中中文摘要映射，最后回退到通用中文归类")
    lines.append("")
    return "\n".join(lines)


def main() -> int:
    args = parse_args()
    mapping = load_mapping()
    previous_tag = find_previous_tag(args.current_tag)
    ref_range = release_range(args.current_tag, previous_tag)
    raw_commits = collect_commits(ref_range, mapping)
    commits = deduplicate_summaries(raw_commits)
    output_path = Path(args.output)
    output_path.parent.mkdir(parents=True, exist_ok=True)
    output_path.write_text(build_markdown(args.current_tag, previous_tag, commits, mapping, len(raw_commits)), encoding="utf-8")
    return 0


if __name__ == "__main__":
    sys.exit(main())
