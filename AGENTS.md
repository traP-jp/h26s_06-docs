# INSTRUCTIONS

- refer `apps/client/AGENTS.md` (if working on `apps/client`)
- refer `apps/server/AGENTS.md` (if working on `apps/server`)

## General Rules

- ユーザーが言及したディレクトリ以外に編集をする場合は、その変更が必要な理由を明示し、ユーザーに確認を取る。
- mock や 開発用の クライアント / サーバー はなるべく起動せず、静的チェックを積極的に活用する。
  - やむを得ない場合は、理由を明示しユーザーに確認を取る。
  - ユーザーに許可され、起動した際は、目的の達成後に必ず停止させる。
- ドキュメントと異なる指示があった場合、できる限りドキュメントを指示に合わせて修正する。
  - その際、変更点を追記するよりも、既存の内容を修正する方を優先する。
- ファイル・ディレクトリは責務によって構造的に分割する。
- 差分を最小化することに固執せず、機能追加・修正に強いコードにする。
- Mock のコードは参考程度にとどめ、コピー & ペーストは絶対に行わない。

## Git / PR Rules

- For development work, always create a new branch before editing code.
- Branch names must start with a type prefix such as `feat/`, `fix/`, or `chore/`.
- Before editing code, AI agents must ask the user whether the work should be done in a git worktree.
- Commit messages must be a single line and start with a Conventional Commits prefix such as `fix:`, `feat:`, or `chore:`.
- PR titles must follow the same format as commit messages, starting with a prefix such as `fix:`, `feat:`, or `chore:`.
- PR descriptions must be short, clean, and focused on the key points.

## Directories

./
├ apps
| ├ client
| └ server
(others)

## Detailed Documents

./docs
├ specs.md 全体の設計の概要
├ endpoints.md API エンドポイント / SSE ペイロード の仕様
├ client
| ├ core.md 基本の構造
| ├ note.md 実装における注意点
| ├ plan.md 実装計画
| └ topology.md グラフのトポロジーに関する設計
└ server
  ├ plan.md 実装計画
  └ specs.md 詳細な設計
