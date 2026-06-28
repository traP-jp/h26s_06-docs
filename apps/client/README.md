## Tools

### ローカル実行

```bash
bun dev
```

### モックサーバー

- `?demo=1` を付与してアクセスすると、traQ に接続せず、テスト用のデータを利用できる。

####　起動

```bash
bun mock:up
```

#### 停止

```bash
bun mock:down
```

### Cloud Run

production image は `$PORT` (default `5173`) で `dist` を配信する。
`GCLOUD_BACKEND_PROXY=true` の場合のみ、`/api` を `http://localhost:8080` に転送する。

Cloud Run では client container に `GCLOUD_BACKEND_PROXY=true` を設定する。
