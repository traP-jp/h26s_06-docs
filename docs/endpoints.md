## Endpoints

- `/api/auth/login` OAuth2. 0のログインURL
- `/api/auth/callback` OAuth2. 0のコールバックURL
- `/api/auth/logout` ログアウト
- `/api/events`　SSE 配信
- `/api/me` ユーザー情報取得


## SSE Payloads

### 3.1 `init` イベント (初期化データ)

新規クライアント接続時に一度だけ送信される、全チャンネルの構造データ。各チャンネルの `score` は小数第3位に丸めて配信する。

```json
event: init
data: {
  "channels": {
    "grand_root": { "id": "grand_root", "parentId": "", "children": ["root_ch_1", "root_ch_2"] },
    "root_ch_1": { "id": "root_ch_1", "parentId": "grand_root", "children": ["sub_ch_10"], "score": 1.234 }
  }
}

```

### 3.2 `trigger` イベント (インパルス配信)

投稿（波紋）や移動（ビーム）が発生した瞬間に即時ブロードキャストされる軽量トリガー。キー名を短縮し帯域を節約する。

**投稿 (MessageCreated) 時:**

```json
event: trigger
data: {"type": "msg", "ch": "sub_ch_10"}

```

**移動 (ChannelWatched) 時:**

```json
event: trigger
data: {"type": "mov", "usr": "user_hash_123", "from": "sub_ch_10", "to": "sub_ch_11"}

```

### 3.3 `sync` イベント (定期同期)

30秒ごとにバックエンドで計算している相対的な「盛り上がり度」を配信し、フロントエンドの自律減衰によるズレを補正する。値は固定上限を持たない raw score で、小数第3位に丸めて配信し、フロントエンド側で表示用の相対値に正規化する。 ./server/specs.md に記載のアルゴリズムに基づいて抽出された差分のみを送信する。

```json
event: sync
data: {
  "ts": 1719300000,
  "deltas": {
    "sub_ch_10": 1.734,
    "root_ch_1": 0.812
  }
}

```
