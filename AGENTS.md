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

## Directories

./
├ apps
| ├ client
| └ server
(others)

## Detailed Documents

./docs
├ specs.md 全体の設計の概要
├ client
| ├ core.md 基本の構造
| ├ note.md 実装における注意点
| ├ plan.md 実装計画
| └ topology.md グラフのトポロジーに関する設計
└ server
  ├ plan.md 実装計画
  └ specs.md 詳細な設計
