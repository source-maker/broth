# OAuth 機能: 既存 JWT への外部 Go パッケージ統合に関する調査

## 概要

Broth フレームワークに OAuth 機能を追加するにあたり、既に実装済みの JWT 基盤（`golang-jwt/jwt/v5`）に外部の Go パッケージをアタッチする方式で実現する。本ドキュメントは実現性の調査結果と推奨ライブラリの選定をまとめたものである。

## 現状の JWT 実装

### 実装箇所

| ファイル | 内容 |
|---|---|
| `auth/token.go` | `TokenService`（生成・検証）、`TokenClaims`、`TokenPair` |
| `auth/token_test.go` | JWT の生成・検証・有効期限・改ざん検知テスト |
| `middleware/auth.go` | Bearer トークン認証 + セッション認証ミドルウェア |
| `middleware/authorize.go` | `RequireAuth`, `RequireRole` 認可ミドルウェア |
| `docs/SECURITY_DESIGN.md` sec 4.4 | OAuth2 拡張ポイント（インターフェース定義のみ） |

### 現在の技術スタック

- **署名アルゴリズム**: HMAC-SHA256（`jwt.SigningMethodHS256`）
- **ライブラリ**: `github.com/golang-jwt/jwt/v5` v5.3.1（ADR-SEC006 で採用決定済み）
- **トークン構成**: AccessToken（JWT）+ RefreshToken（crypto/rand による乱数）
- **認証方式**: SSR（セッション）/ API（Bearer）の自動判別

### 既存の OAuth2 拡張ポイント（SECURITY_DESIGN.md sec 4.4）

`OAuth2Provider` インターフェースと `OAuth2Handler` が設計されているが、**未実装**:

```go
type OAuth2Provider interface {
    Name() string
    AuthURL(state string) string
    Exchange(ctx context.Context, code string) (*OAuth2UserInfo, error)
}
```

---

## 調査対象ライブラリ

### 1. OAuth クライアント（ソーシャルログイン用）

#### 1-1. `golang.org/x/oauth2`（推奨）

| 項目 | 詳細 |
|---|---|
| **リポジトリ** | [github.com/golang/oauth2](https://github.com/golang/oauth2) |
| **GitHub Stars** | ~5,500+ |
| **ライセンス** | BSD-3-Clause |
| **最終更新** | 2026年1月（アクティブにメンテナンス中） |
| **インポート数** | 46,692 パッケージ |
| **Go バージョン** | Go 1.17+ |

**特徴**:
- Go チーム公式の OAuth2 クライアントライブラリ
- Authorization Code Flow、Client Credentials Flow、JWT Bearer Flow をサポート
- PKCE（Proof Key for Code Exchange）対応
- Google、GitHub 等の主要プロバイダのエンドポイント内蔵
- `golang.org/x/oauth2/endpoints` パッケージで新規プロバイダを簡易追加可能

**JWT 統合性**: クライアントライブラリのため JWT 生成には関与しない。プロバイダからの認可コードをアクセストークンに交換し、ユーザー情報を取得するのが役割。**Broth の既存 JWT 基盤とは自然に共存可能**。

**Broth への適合性**: **最適**
- Go 標準ライブラリに準ずる品質
- P1 原則（Go イディオム）に完全合致
- 最小限の依存で Social Login を実現
- SECURITY_DESIGN.md の `OAuth2Provider` インターフェース実装に直接使用可能

#### 1-2. `go-pkgz/auth`

| 項目 | 詳細 |
|---|---|
| **リポジトリ** | [github.com/go-pkgz/auth](https://github.com/go-pkgz/auth) |
| **GitHub Stars** | ~1,300 |
| **ライセンス** | MIT |
| **最終更新** | 2025年3月（v2.1.1） |
| **JWT ライブラリ** | v2 で `golang-jwt/jwt/v5` に移行済み |

**特徴**:
- マルチプロバイダ対応（Google, GitHub, Facebook, Microsoft, Twitter, Apple, Discord, Telegram 等）
- JWT + XSRF 保護の統合
- ClaimsUpdater / Validator による柔軟なカスタマイズ
- Dev Provider によるローカルテスト支援

**JWT 統合性**: `golang-jwt/jwt/v5` を直接使用しており、Broth の JWT 基盤と同じライブラリを使用。

**Broth への適合性**: **非推奨**
- Broth 独自のアーキテクチャ（ミドルウェア、セッション管理、コンテキスト自動判別）と設計思想が衝突
- フレームワーク全体を入れ替えるレベルの統合が必要
- 「バッテリー同梱」フレームワークに別のバッテリー同梱ライブラリを入れるのは冗長

#### 1-3. `markbates/goth`

| 項目 | 詳細 |
|---|---|
| **リポジトリ** | [github.com/markbates/goth](https://github.com/markbates/goth) |
| **GitHub Stars** | ~6,500 |
| **ライセンス** | MIT |
| **最終更新** | 2025年8月（v1.82.0） |
| **対応プロバイダ** | 70+ |

**特徴**:
- 70 以上の OAuth プロバイダに対応（Google, GitHub, Apple, Discord, Slack 等）
- クリーンで Go イディオムに沿った API
- `gothic` ヘルパーパッケージによる HTTP ハンドラ統合
- カスタムプロバイダのサポート

**JWT 統合性**: OAuth クライアント専用で JWT 生成には関与しない。認証後に独自 JWT を発行する方式で使用。

**Broth への適合性**: **非推奨**
- メンテナー後継問題あり（「goth needs a new maintainer」ブログ記事公開済み）
- セッションベース前提で JWT ベースの設計ではない
- OIDC 固有機能（ID トークン検証、ディスカバリ）が不足
- `x/oauth2` + `go-oidc` の方が Broth の設計に合致

---

### 2. OAuth サーバー（認可サーバー構築用）

#### 2-1. `ory/fosite`

| 項目 | 詳細 |
|---|---|
| **リポジトリ** | [github.com/ory/fosite](https://github.com/ory/fosite) |
| **GitHub Stars** | ~2,500 |
| **ライセンス** | Apache-2.0 |
| **最終バージョン** | v0.49.0 |
| **JWT ライブラリ** | 独自実装（`go-jose` ベース、旧 `dgrijalva/jwt-go` から移行） |

**特徴**:
- RFC 6749/6819 準拠のセキュリティファースト設計
- Authorization Code, Client Credentials, Implicit, Refresh Token, JWT Bearer, Device Code, PAR をサポート
- OpenID Connect Core 1.0 対応
- `jwt.Signer` インターフェースと `JWTClaimsContainer` によるカスタマイズ

**JWT 統合性**: **直接統合不可**
- Fosite は独自の JWT 実装（`github.com/ory/fosite/token/jwt`）を使用
- `golang-jwt/jwt/v5` を直接プラグインする設計ではない
- カスタム JWT 生成には Fosite 独自の `jwt.Signer` インターフェースの実装が必要
- Broth の `TokenService` をそのまま使うことはできない

**Broth への適合性**: **Phase 2 以降で検討**
- Broth が独自の OAuth2 認可サーバーになる場合に有力
- ただし重量級（依存関係が多い）で P4（YAGNI）原則に反する可能性
- JWT 基盤の二重管理が発生するリスク

#### 2-2. `go-oauth2/oauth2`

| 項目 | 詳細 |
|---|---|
| **リポジトリ** | [github.com/go-oauth2/oauth2](https://github.com/go-oauth2/oauth2) |
| **GitHub Stars** | ~3,600 |
| **ライセンス** | MIT |
| **最終バージョン** | v4 |
| **JWT ライブラリ** | `golang-jwt/jwt` をネイティブサポート |

**特徴**:
- 軽量な OAuth2 サーバーライブラリ
- `generates.NewJWTAccessGenerate()` で JWT アクセストークンを生成
- 多数のストレージバックエンド対応（PostgreSQL, Redis, MongoDB 等）
- `AccessGenerate` インターフェースによるカスタムトークン生成

**JWT 統合性**: **統合可能**
- `golang-jwt/jwt` をネイティブに使用
- `generates.NewJWTAccessGenerate("", []byte("secret"), jwt.SigningMethodHS512)` で直接設定
- Broth の `TokenService` のロジックを `AccessGenerate` として組み込み可能

**Broth への適合性**: **Phase 2 で有力候補**
- OAuth2 認可サーバー構築時に最も統合しやすい
- `golang-jwt/jwt` 共通のため JWT 基盤を共有可能
- Fosite より軽量で YAGNI 原則に合致

---

### 3. OpenID Connect（OIDC）ライブラリ

#### 3-1. `coreos/go-oidc`（推奨）

| 項目 | 詳細 |
|---|---|
| **リポジトリ** | [github.com/coreos/go-oidc](https://github.com/coreos/go-oidc) |
| **GitHub Stars** | ~2,300 |
| **ライセンス** | Apache-2.0 |
| **最終バージョン** | v3.17.0 |
| **インポート数** | 1,787 パッケージ |

**特徴**:
- `golang.org/x/oauth2` の OIDC 拡張
- プロバイダディスカバリ（`.well-known/openid-configuration`）
- ID トークンの検証（署名、有効期限、nonce）
- Remote / Static KeySet によるキー管理

**JWT 統合性**: クライアント側ライブラリのため、JWT 生成には関与しない。OIDC プロバイダから受け取った ID トークン（JWT）の検証を担当。**Broth の JWT 基盤と競合しない**。

**Broth への適合性**: **最適**
- `golang.org/x/oauth2` との組み合わせがデファクトスタンダード
- Google, Azure AD 等の OIDC プロバイダ対応に必要
- 軽量で最小限の依存

#### 3-2. `zitadel/oidc`

| 項目 | 詳細 |
|---|---|
| **リポジトリ** | [github.com/zitadel/oidc](https://github.com/zitadel/oidc) |
| **GitHub Stars** | ~1,700 |
| **ライセンス** | Apache-2.0 |
| **最終バージョン** | v3.45.4（2026年2月） |
| **Go バージョン** | Go 1.24+ |

**特徴**:
- OIDC クライアント（RP）とサーバー（OP）の両方を提供
- OpenID Foundation 認定（Basic / Config プロファイル）
- OpenTelemetry 統合
- Code Flow, Client Credentials, Refresh Tokens, Discovery, JWT Profile, PKCE, Token Exchange, Device Authorization をサポート

**JWT 統合性**: 独自の JWT 実装を使用。`Storage` と `KeySet` インターフェースにより間接的なカスタマイズは可能。

**Broth への適合性**: **非推奨（Phase 1）**
- OP（サーバー）機能を含むため、クライアントのみの用途にはオーバースペック
- `coreos/go-oidc` + `x/oauth2` の組み合わせの方がシンプル
- Broth が OIDC プロバイダになる場合（Phase 2+）には有力候補

---

## ライブラリ比較マトリクス

| ライブラリ | 種別 | Stars | ライセンス | golang-jwt/v5 互換 | 最終リリース | OIDC |
|---|---|---|---|---|---|---|
| **golang.org/x/oauth2** | クライアント | ~5,800 | BSD-3 | N/A（クライアント） | 2026/01 | No |
| **coreos/go-oidc** | OIDC クライアント | ~2,300 | Apache-2.0 | No（独自検証） | 2025/11 | クライアントのみ |
| **go-oauth2/oauth2** | サーバー SDK | ~3,600 | MIT | **Yes（ネイティブ）** | 2025/08 | No |
| **ory/fosite** | サーバー SDK | ~2,500 | Apache-2.0 | No（go-jose） | 2024/12 | Yes |
| **zitadel/oidc** | OIDC 双方向 | ~1,800 | Apache-2.0 | No（独自JOSE） | 2026/02 | **認定済み** |
| **markbates/goth** | クライアント（ソーシャル） | ~6,500 | MIT | N/A | 2025/08 | 部分的 |
| **go-pkgz/auth** | クライアント + JWT MW | ~1,300 | MIT | **Yes（v2）** | 2025/03 | No |

---

## 実現性評価

### アーキテクチャパターン: OAuth + 既存 JWT の統合

```
ユーザー
  │
  ├─ (1) /auth/{provider}/login → AuthURL 生成 → プロバイダへリダイレクト
  │
  ├─ (2) /auth/{provider}/callback ← 認可コード受領
  │         │
  │         ├─ golang.org/x/oauth2: コード → アクセストークン交換
  │         ├─ coreos/go-oidc: ID トークン検証（OIDC の場合）
  │         ├─ ユーザー情報取得（UserInfo エンドポイント or ID トークン）
  │         │
  │         ├─ onLogin コールバック: ユーザー解決/作成
  │         │     └─ 既存の account モジュールのサービス層を使用
  │         │
  │         └─ Broth 独自の JWT 生成（既存の TokenService を使用）
  │               └─ auth/token.go: GenerateAccessToken()
  │
  └─ (3) 以降の API リクエスト: 既存の Bearer トークン認証
              └─ middleware/auth.go: ValidateAccessToken()
```

**結論: 完全に実現可能**

- OAuth クライアントライブラリ（`x/oauth2` + `go-oidc`）はプロバイダとのトークン交換のみを担当
- ユーザー認証後のアプリケーション内トークンは既存の `TokenService` がそのまま発行
- JWT 基盤の変更は不要。外部ライブラリは「プロバイダとの通信」を担当するだけ

### トークン戦略（既存設計との整合）

| トークン種別 | 形式 | 検証方式 | 失効方式 |
|---|---|---|---|
| Access Token | JWT（短命: 15分） | ステートレス（golang-jwt による署名検証） | 自然失効 |
| Refresh Token | Opaque（乱数文字列） | ステートフル（DB 照合） | DB から削除 |
| ID Token（OIDC） | JWT | プロバイダの JWKS で検証（go-oidc） | N/A（使い捨て） |
| プロバイダ Access Token | Opaque/JWT | プロバイダ API で使用 | セッション終了時に破棄 |

この戦略は既存の `TokenPair`（AccessToken + RefreshToken）構造と完全に一致する。

### Broth の既存設計との整合性

| 設計要素 | 適合性 |
|---|---|
| `OAuth2Provider` インターフェース（SECURITY_DESIGN.md） | `x/oauth2` で直接実装可能 |
| `OAuth2Handler`（コールバック処理） | 既存設計をそのまま実装 |
| `OAuth2UserInfo`（ユーザー情報） | プロバイダの UserInfo から変換 |
| `onLogin` コールバック | account モジュールの Service 層で実装 |
| `TokenService`（JWT 生成） | 変更不要、OAuth ログイン後にそのまま使用 |
| コンテキスト自動判別 | OAuth コールバックは SSR フローのため影響なし |

---

## 推奨構成

### Phase 1: ソーシャルログイン（OAuth クライアント）

| ライブラリ | 役割 | バージョン |
|---|---|---|
| `golang.org/x/oauth2` | OAuth2 クライアント（認可コード交換） | latest |
| `github.com/coreos/go-oidc/v3` | OIDC ID トークン検証 | v3.17+ |
| `github.com/golang-jwt/jwt/v5` | **既存** - アプリ内 JWT 生成/検証 | v5.3.1（変更なし） |

**追加依存**: 2 パッケージのみ（`x/oauth2` は Go 準標準）

**実装スコープ**:
1. `auth/oauth2.go` - `OAuth2Provider` インターフェースの実装
2. `auth/providers/google.go` - Google プロバイダ
3. `auth/providers/github.go` - GitHub プロバイダ
4. ハンドラー: `/auth/{provider}/login`, `/auth/{provider}/callback`
5. `config/` に OAuth2 設定の追加
6. DB マイグレーション: `oauth_accounts` テーブル（プロバイダ + プロバイダ ID + ユーザー ID のマッピング）

### Phase 2（将来）: OAuth2 認可サーバー

Broth アプリケーション自体が OAuth2 プロバイダとなる場合:

| ライブラリ | 役割 |
|---|---|
| `github.com/go-oauth2/oauth2/v4` | OAuth2 認可サーバー |
| `github.com/golang-jwt/jwt/v5` | JWT 基盤を共有 |

**注意**: Phase 2 は YAGNI 原則により、明確な要件が出るまで着手しない。

---

## リスクと考慮事項

### 技術的リスク

1. **CSRF 対策**: OAuth state パラメータの安全な生成・検証が必要（`crypto/rand` 使用）
2. **セッション固定攻撃**: OAuth コールバック後のセッション再生成（既存の `sess.Regenerate()` で対応可能）
3. **トークン混同**: プロバイダのアクセストークンとアプリ内 JWT を混同しない設計が必要

### 設計上の考慮事項

1. **アカウントリンク**: 既存アカウント（メール/パスワード）と OAuth アカウントの紐付けロジック
2. **マルチプロバイダ**: 1 ユーザーが複数の OAuth プロバイダでログインする場合の処理
3. **メールの一意性**: OAuth プロバイダから取得したメールと既存ユーザーのメールの照合ポリシー

---

## まとめ

| 観点 | 評価 |
|---|---|
| **実現性** | 完全に実現可能。既存 JWT 基盤への影響なし |
| **推奨方式** | `x/oauth2` + `go-oidc` をクライアントとして使用し、認証後のトークン発行は既存 `TokenService` |
| **追加依存** | 2 パッケージのみ（Go 準標準 + デファクトスタンダード） |
| **P1 適合** | Go イディオムに完全合致（Go チーム公式 / 準標準ライブラリ） |
| **P4 適合** | Phase 1 はソーシャルログインのみ、認可サーバーは将来に留保 |
| **既存設計** | SECURITY_DESIGN.md の拡張ポイントをそのまま活用可能 |
