# Broth -- 統合設計判断ログ (DECISION_LOG)

> **バージョン**: 0.1.0-draft
> **最終更新**: 2026-02-08
> **ステータス**: 初期設計
> **ADR 総数**: 37件（ADR-M006, ADR-D007, ADR-VER001 を含む）

---

## 目次

1. [概要](#1-概要)
2. [設計優先度の原則](#2-設計優先度の原則)
3. [ADR 一覧表（サマリ）](#3-adr-一覧表サマリ)
4. [コアアーキテクチャ（ADR-001〜005）](#4-コアアーキテクチャadr-001005)
5. [モジュール設計（ADR-M001〜M006）](#5-モジュール設計adr-m001m006)
6. [プロジェクト構造・DX（ADR-PS001〜PS007）](#6-プロジェクト構造dxadr-ps001ps007)
7. [並列処理・ジョブ（ADR-C001〜C005）](#7-並列処理ジョブadr-c001c005)
8. [セキュリティ（ADR-SEC001〜SEC006）](#8-セキュリティadr-sec001sec006)
9. [データアクセス（ADR-D001〜D007）](#9-データアクセスadr-d001d007)
10. [クロスレビュー記録](#10-クロスレビュー記録)
11. [将来検討事項（Future / Pending）](#11-将来検討事項future--pending)
12. [差別化機能サマリ](#12-差別化機能サマリ)
13. [競合分析サマリ](#13-競合分析サマリ)
14. [バージョニング方針](#14-バージョニング方針)

---

## 1. 概要

本ドキュメントは、Broth フレームワークの全設計ドキュメントから抽出した **Architecture Decision Records (ADR)** を統合・整理したものである。6つの設計書に散在する36件のADRを一箇所に集約し、ドキュメント間の整合性を横断的に検証する。

### 対象ドキュメント

| # | ドキュメント | ADR 数 | カテゴリ |
|---|---|---|---|
| 1 | [ARCHITECTURE.md](./ARCHITECTURE.md) | 5 | コアアーキテクチャ |
| 2 | [MODULE_DESIGN.md](./MODULE_DESIGN.md) | 5 | モジュール設計 |
| 3 | [PROJECT_STRUCTURE.md](./PROJECT_STRUCTURE.md) | 7 | プロジェクト構造・DX |
| 4 | [CONCURRENCY_DESIGN.md](./CONCURRENCY_DESIGN.md) | 5 | 並列処理・ジョブ |
| 5 | [SECURITY_DESIGN.md](./SECURITY_DESIGN.md) | 6 | セキュリティ |
| 6 | [DATA_ACCESS_DESIGN.md](./DATA_ACCESS_DESIGN.md) | 7 | データアクセス |

---

## 2. 設計優先度の原則

全てのADRは以下の4段階の優先度原則に基づいて判断されている。上位の原則が下位に優先する。

| 優先度 | 原則 | 説明 | 対応するADR例 |
|---|---|---|---|
| **P1** | Go イディオムへの忠誠 | 標準ライブラリ最大活用、リフレクション最小限、`interface{}` 排除、コンストラクタ注入 | ADR-001, ADR-002, ADR-003, ADR-D001 |
| **P2** | AI 収束性の最大化 | 構造の規約化、「どこに何を書くか」の一意決定、命名規約の明文化 | ADR-M001, ADR-PS001〜PS007, ADR-004 |
| **P3** | チーム運用オーバーヘッドの最小化 | 7+2人チーム規模での管理可能性、デプロイの単純さ、Secure by Default | ADR-C001, ADR-SEC001, ADR-SEC005 |
| **P4** | 将来拡張性 (YAGNI) | Phase 1 でシンプルに、Phase 2 以降の拡張パスを確保 | ADR-M002, ADR-C002, ADR-C005 |

---

## 3. ADR 一覧表（サマリ）

### コアアーキテクチャ

| ADR ID | タイトル | 決定 | 優先度原則 |
|---|---|---|---|
| ADR-001 | 独自 Context 型を作らない | `context.Context` + ジェネリクス型安全アクセサ | P1 |
| ADR-002 | DI コンテナを使わない | コンストラクタ注入（`New*` 関数） | P1 |
| ADR-003 | リフレクションの使用方針 | form / config / admin のみ許可、他はコード生成 | P1 |
| ADR-004 | ビジネスロジック層の明示的定義 | `service.go` を唯一の置き場所に規定 | P2 |
| ADR-005 | パッケージ構造の深さ | 2〜3階層（`modules/{name}/internal/store/` まで） | P1, P2 |

### モジュール設計

| ADR ID | タイトル | 決定 | 優先度原則 |
|---|---|---|---|
| ADR-M001 | モジュールのディレクトリ構造 | 2階層 + internal | P1, P2 |
| ADR-M002 | モジュール間通信 | Phase 1: 直接参照、Phase 2: イベント | P4 |
| ADR-M003 | 共通型の管理方針 | `modules/shared/` パッケージに配置 | P2 |
| ADR-M004 | Repository インターフェースの配置場所 | 専用ファイル（`repository.go`）に配置 | P2 |
| ADR-M005 | forms.go の位置づけ | HTTPレイヤーに属し、バリデーションはドメインに委譲 | P1, P2 |
| ADR-M006 | Django apps 概念の不採用（Phase 1） | 現在の `modules/` で十分。大規模化時に再検討 | P1, P4 |

### プロジェクト構造・DX

| ADR ID | タイトル | 決定 | 優先度原則 |
|---|---|---|---|
| ADR-PS001 | プロジェクトルートの `broth/` 廃止 | Go モジュール外部依存として分離 | P1 |
| ADR-PS002 | `cmd/myapp/` vs `cmd/server/` | `cmd/myapp/`（プロジェクト名一致） | P2 |
| ADR-PS003 | `db/migrations/` vs `migrations/` | `db/migrations/`（DB関連ファイル集約） | P2 |
| ADR-PS004 | `config/` ディレクトリの導入 | 設定構造体・ルーティング・ミドルウェアを集約 | P2 |
| ADR-PS005 | テンプレートの二重配置 | 共通 `templates/` + モジュール固有 | P2 |
| ADR-PS006 | `.broth/rules.md` のコミット対象 | git管理対象とする | P2 |
| ADR-PS007 | Makefile の採用 | タスクランナーとして採用 | P3 |

### 並列処理・ジョブ

| ADR ID | タイトル | 決定 | 優先度原則 |
|---|---|---|---|
| ADR-C001 | インメモリとDBの2層構造 | 用途に応じたインメモリ + DB永続化 | P1, P3 |
| ADR-C002 | DB ポーリング vs LISTEN/NOTIFY | Phase 1: ポーリング（SELECT FOR UPDATE SKIP LOCKED） | P4 |
| ADR-C003 | ジョブのシリアライズ方式 | JSON（`encoding/json`）+ JSONB | P1 |
| ADR-C004 | WebSocket ライブラリの選定 | `nhooyr.io/websocket` 推奨、コアに含めず | P1 |
| ADR-C005 | スケジューラのリーダー選出方式 | DB 行ロック | P3, P4 |

### セキュリティ

| ADR ID | タイトル | 決定 | 優先度原則 |
|---|---|---|---|
| ADR-SEC001 | コンテキスト自動適用 | リクエスト単位でSSR/APIを自動判定 | P3 |
| ADR-SEC002 | CSRF パターン | Synchronizer Token パターン | P3 |
| ADR-SEC003 | パスワードハッシュ | bcrypt をデフォルト採用 | P1 |
| ADR-SEC004 | レート制限アルゴリズム | Token Bucket（`golang.org/x/time/rate`） | P1 |
| ADR-SEC005 | セキュリティヘッダ | デフォルトで全て有効化 | P3 |
| ADR-SEC006 | JWT 実装方針 | `golang-jwt/jwt` を採用（セキュリティライブラリは例外的に許容） | P1 |

### データアクセス

| ADR ID | タイトル | 決定 | 優先度原則 |
|---|---|---|---|
| ADR-D001 | Bob を推奨データアクセスライブラリとして採用 | Bob（database-first コード生成）を `broth/db` で薄くラップ | P1 |
| ADR-D002 | マイグレーションツール | goose を broth/migrate でラップ | P1 |
| ADR-D003 | Admin 画面の方式 | コード生成ベース（リフレクション不使用） | P1 |
| ADR-D004 | テンプレートエンジン | html/template 拡張（外部エンジン不使用） | P1 |
| ADR-D005 | broth/form でのリフレクション使用 | 許可（ADR-003 の許可範囲内） | P1 |
| ADR-D006 | `broth generate model` の実装戦略 | bobgen-psql をラップ | P4 |
| ADR-D007 | GORM 不採用 | `interface{}` 多用・リフレクション依存が設計原則と衝突 | P1 |

---

## 4. コアアーキテクチャ（ADR-001〜005）

**出典**: [ARCHITECTURE.md](./ARCHITECTURE.md) sec 6

### ADR-001: 独自 Context 型を作らない

| 項目 | 内容 |
|---|---|
| **ステータス** | 承認済み |
| **カテゴリ** | コアアーキテクチャ |
| **関連ドキュメント** | ARCHITECTURE.md sec 3.1, SECURITY_DESIGN.md sec 3.3 |
| **優先度原則** | P1（Go イディオム） |

**状況**: Gin/Echo は独自の `Context` 型を提供し、リクエスト/レスポンスの操作を集約する。

**決定**: 独自 Context 型を作らず、`context.Context` + ジェネリクスベースの型安全アクセサ（`ctxKey[T]`）を使う。

**根拠**:
- `net/http` のハンドラシグネチャ `(w http.ResponseWriter, r *http.Request)` との互換性を維持
- 標準ライブラリのミドルウェアをそのまま利用可能
- 独自型は学習コストを増やし、エコシステムとの互換性を損なう
- ジェネリクス（Go 1.18+）により `interface{}` なしで型安全なコンテキスト値アクセスが可能

**影響**: SECURITY_DESIGN.md の `requestContextKey` や `auth.User` のコンテキスト格納もこのパターンに準拠する。

---

### ADR-002: DI コンテナを使わない

| 項目 | 内容 |
|---|---|
| **ステータス** | 承認済み |
| **カテゴリ** | コアアーキテクチャ |
| **関連ドキュメント** | ARCHITECTURE.md sec 3.2, MODULE_DESIGN.md sec 5, PROJECT_STRUCTURE.md sec 3.1 |
| **優先度原則** | P1（Go イディオム） |

**状況**: NestJS は DI コンテナで依存を管理する。Go にも Wire, Fx 等のDIライブラリがある。

**決定**: DI コンテナを使わない。コンストラクタ注入（`New*` 関数）で依存を解決する。

**根拠**:
- Go の明示性の文化に合致。「何が注入されるか」がコードを読むだけで分かる
- ターゲット規模（7+2人チーム）では手動の依存組み立て（`main.go` のワイヤリング）で十分管理可能
- DI コンテナは実行時エラーを発生させうるが、コンストラクタ注入はコンパイル時に検出する
- Wire 等のコード生成DIは、依存グラフが複雑化した場合のエスケープハッチとしてドキュメントで案内

**影響**: MODULE_DESIGN.md の `NewModule` パターン、PROJECT_STRUCTURE.md の `cmd/myapp/main.go` のワイヤリングがこの判断に直接基づく。Stage 3（モジュール9個以上）で Wire 導入を検討する旨が PROJECT_STRUCTURE.md sec 7.1 に記載。

---

### ADR-003: リフレクションの使用方針

| 項目 | 内容 |
|---|---|
| **ステータス** | 承認済み |
| **カテゴリ** | コアアーキテクチャ |
| **関連ドキュメント** | ARCHITECTURE.md sec 6, DATA_ACCESS_DESIGN.md ADR-D005 |
| **優先度原則** | P1（Go イディオム） |

**状況**: ORMやフォームバインディングでリフレクションを使うのが一般的。

**決定**: リフレクションは以下の場面でのみ許可する。

| 許可する場面 | 理由 |
|---|---|
| `broth/form` のフォームバインディング | 構造体タグの読み取りは Go の標準的パターン（`encoding/json` と同等） |
| `broth/config` の環境変数バインディング | 同上 |
| `broth/admin` のモデル一覧表示 | 管理画面のUIで構造体フィールドを動的に列挙する必要がある |

それ以外では `go generate` によるコード生成を優先する。

**影響**: ADR-D001（Bob 採用 -- コード生成ベースでリフレクション不使用）、ADR-D003（Admin画面コード生成）、ADR-D005（form でのリフレクション許可）、ADR-D007（GORM 不採用）と整合する。

---

### ADR-004: ビジネスロジック層の明示的定義

| 項目 | 内容 |
|---|---|
| **ステータス** | 承認済み |
| **カテゴリ** | コアアーキテクチャ |
| **関連ドキュメント** | ARCHITECTURE.md sec 3.2, MODULE_DESIGN.md sec 2, PROJECT_STRUCTURE.md sec 3.3 |
| **優先度原則** | P2（AI 収束性） |

**状況**: Go のWebアプリケーションではビジネスロジックの置き場所が曖昧になりがち。

**決定**: `service.go` をビジネスロジックの公式の置き場所と定義する。

**根拠**:
- Rails の「fat model / fat controller」問題を回避
- Django の「views.py にロジックが肥大化する」問題を回避
- 1モジュール1サービス構造体を原則とし、ロジックの分散を防ぐ
- テスタビリティ: サービスの依存はインターフェースで注入されるためモック差し替えが容易

**影響**: MODULE_DESIGN.md の全モジュール構造、PROJECT_STRUCTURE.md のファイル配置ルール、`.broth/rules.md` の生成ルールがこの判断に基づく。

---

### ADR-005: パッケージ構造の深さ

| 項目 | 内容 |
|---|---|
| **ステータス** | 承認済み |
| **カテゴリ** | コアアーキテクチャ |
| **関連ドキュメント** | ARCHITECTURE.md sec 6, MODULE_DESIGN.md ADR-M001 |
| **優先度原則** | P1（Go イディオム）, P2（AI 収束性） |

**状況**: Go は「フラットなパッケージ」を好む文化がある一方、モジュール境界の明確化には一定の深さが必要。

**決定**: 2〜3階層のネストを許容する。具体的には `modules/{name}/internal/store/` まで。

**根拠**:
- Go の `internal/` ディレクトリによるアクセス制御を活用するには最低2階層が必要
- 3階層は Go の大規模プロジェクト（Kubernetes, Docker）でも一般的に受け入れられている深さ
- 4階層以上は避ける。過度なネストは Go の文化に反し、可読性を損なう
- `modules/` プレフィックスにより「フレームワークコード」と「アプリケーションコード」を視覚的に分離

**影響**: ADR-M001 と直接整合。PROJECT_STRUCTURE.md のディレクトリ構成がこの制約に従う。

---

## 5. モジュール設計（ADR-M001〜M006）

**出典**: [MODULE_DESIGN.md](./MODULE_DESIGN.md) sec 8

### ADR-M001: モジュールのディレクトリ構造 -- フラット vs ネスト

| 項目 | 内容 |
|---|---|
| **ステータス** | 承認済み |
| **カテゴリ** | モジュール設計 |
| **関連ドキュメント** | MODULE_DESIGN.md sec 2, ARCHITECTURE.md ADR-005 |
| **優先度原則** | P1（Go イディオム）, P2（AI 収束性） |

**状況**: ディレクトリ構造としてフラット/1階層/2階層+internal/深いネストの4選択肢。

**決定**: **2階層 + internal**（`modules/account/internal/store/`）を採用。

**根拠**:
- Go の `internal/` によるアクセス制御には最低2階層が必要
- `modules/` プレフィックスで「アプリコード」と「フレームワークコード」が視覚的に分離される
- 3階層（`modules/account/internal/store/`）は Go 大規模プロジェクトでも一般的
- 4階層以上は Go 文化に反する

---

### ADR-M002: モジュール間通信 -- 直接参照 vs メッセージング

| 項目 | 内容 |
|---|---|
| **ステータス** | 承認済み（Phase 1） |
| **カテゴリ** | モジュール設計 |
| **関連ドキュメント** | MODULE_DESIGN.md sec 4, CONCURRENCY_DESIGN.md sec 4 |
| **優先度原則** | P4（将来拡張性 / YAGNI） |

**状況**: モジュール間の通信方法を決定する必要がある。

**決定**: Phase 1 では直接参照（サービスのメソッド呼び出し）を基本とする。イベントベースの疎結合は Phase 2 で導入。

**根拠**:
- 7+2人のチームで、最初からイベント駆動にするのはオーバーエンジニアリング
- Go の関数呼び出しはスタックトレースが明確でデバッグ容易
- 直接参照でも `internal/` による境界制御は機能する
- イベント駆動が必要になった場合（通知、監査ログ等）に部分的に導入する設計余地を残す

---

### ADR-M003: 共通型の管理方針

| 項目 | 内容 |
|---|---|
| **ステータス** | 承認済み |
| **カテゴリ** | モジュール設計 |
| **関連ドキュメント** | MODULE_DESIGN.md sec 7, PROJECT_STRUCTURE.md sec 3.4 |
| **優先度原則** | P2（AI 収束性） |

**状況**: 複数モジュールで使われる型（Money, Pagination等）の配置場所。

**決定**: `modules/shared/` パッケージに配置する。

**根拠**:
- Go は循環 import を禁止するため、共通型を各モジュールに重複定義するのは非効率
- `shared/` は「値オブジェクト」「ヘルパー型」のみに限定し、ビジネスロジックは含めない
- `shared/` が肥大化した場合はモジュール分割の兆候として扱う

---

### ADR-M004: Repository インターフェースの配置場所

| 項目 | 内容 |
|---|---|
| **ステータス** | 承認済み |
| **カテゴリ** | モジュール設計 |
| **関連ドキュメント** | MODULE_DESIGN.md sec 2, ARCHITECTURE.md sec 3.4, DATA_ACCESS_DESIGN.md sec 1.5 |
| **優先度原則** | P2（AI 収束性） |

**状況**: Repository インターフェースを model.go / 専用ファイル / フレームワークジェネリクスのいずれに定義するか。

**決定**: **専用ファイル（`repository.go`）** に配置する。

**根拠**:
- Repository はドメインモデルとは関心事が異なる（データアクセスの抽象）
- 専用ファイルにすることで、1ファイル1関心事の原則に従う
- フレームワーク側のジェネリックインターフェースは、CRUD以外の操作を表現しにくい
- モジュール固有のインターフェースの方が必要最小限のメソッドを定義でき、テスト時のモック作成も容易

---

### ADR-M005: forms.go の位置づけ -- HTTPレイヤー vs ドメインレイヤー

| 項目 | 内容 |
|---|---|
| **ステータス** | 承認済み |
| **カテゴリ** | モジュール設計 |
| **関連ドキュメント** | MODULE_DESIGN.md sec 2, ARCHITECTURE.md sec 3.3, DATA_ACCESS_DESIGN.md sec 6.7 |
| **優先度原則** | P1（Go イディオム）, P2（AI 収束性） |

**状況**: フォーム定義はどのレイヤーに属するか。

**決定**: フォーム定義はHTTPレイヤーに属する。ただしバリデーションはドメインレイヤーに委譲する。

**根拠**:
- フォームはHTTPリクエストのバインディングという HTTP固有の関心事
- 「メールアドレスの形式」「パスワードの最小長」等のルールはドメイン知識
- `forms.go` は入力のパース・バインドを担い、バリデーション自体は `model.go` の `Validate()` に委譲
- API（JSONバインド）とHTMLフォーム（FormDataバインド）で同じバリデーションロジックを共有可能

---

### ADR-M006: Django apps 概念の不採用（Phase 1）

| 項目 | 内容 |
|---|---|
| **ステータス** | 承認済み |
| **カテゴリ** | モジュール設計 |
| **関連ドキュメント** | MODULE_DESIGN.md sec 1-5, ARCHITECTURE.md sec 3 |
| **優先度原則** | P1（Go イディオム）, P4（YAGNI） |

**状況**: Django の apps のように、複数の Feature Module をビジネスドメイン単位でグルーピングする上位概念（`apps/`）を導入すべきか。複数の観点から批判的検証を実施。

**決定**: Phase 1 では apps 概念を導入しない。現在の `modules/` 設計のまま実装を進める。

**根拠**:
1. **Django apps は Python の言語的制約に起因する設計** -- アクセス修飾子の欠如、実行時循環 import 検出、動的型付けによる登録ミスの遅延検出。Go では `internal/`・コンパイル時循環 import 検出・型安全なインターフェース適合で**言語仕様により解決済み**
2. **現在の modules で要望の大部分を達成** -- ドメイン単位の並列配置（`modules/{name}/`）、モジュール間のコード共有（サービス直接参照 / インターフェース疎結合）、暗黙的グローバル副作用の排除（`broth.Module` + コンストラクタ注入）は全て実現済み
3. **ADR-005 との衝突** -- apps 導入でパッケージ深度が最大5階層（`apps/{app}/{module}/internal/store/`）に増加し、2-3階層の原則に違反
4. **P2（AI 収束性）の低下** -- `modules/` と `apps/` の二重構造により「どちらに置くか」の判断ポイントが増加。PROJECT_STRUCTURE.md の「収束性 5/5」目標に反する
5. **P4（YAGNI）** -- Broth のターゲット規模（7±2人チーム）では modules 数が 9 を超えるケースは稀

**再検討のトリガー条件**:
- Module 数が 9 個以上に到達
- チーム規模が 15 人以上に拡大
- 3 つ以上の明確に独立したビジネスドメインが存在

**参考資料**: 設計検討では `broth.AppDefinition` インターフェース、ファサードパターン、段階的移行パス等を評価した。再検討時にはこれらのアプローチを出発点とする。

---

## 6. プロジェクト構造・DX（ADR-PS001〜PS007）

**出典**: [PROJECT_STRUCTURE.md](./PROJECT_STRUCTURE.md) sec 8

### ADR-PS001: プロジェクトルートの `broth/` ディレクトリを廃止

| 項目 | 内容 |
|---|---|
| **ステータス** | 承認済み |
| **カテゴリ** | プロジェクト構造 |
| **関連ドキュメント** | PROJECT_STRUCTURE.md sec 2, ARCHITECTURE.md 付録 |
| **優先度原則** | P1（Go イディオム） |

**状況**: ARCHITECTURE.md の付録では `broth/` ディレクトリにフレームワークコアコードを配置する構造が示されている。

**決定**: フレームワークコアは Go モジュール（`github.com/source-maker/broth`）として外部依存にし、プロジェクトルートに `broth/` ディレクトリは配置しない。

**根拠**:
- フレームワークコードとアプリケーションコードが同一リポジトリに混在すると関心の分離が曖昧
- `go mod` の依存管理でバージョニング・アップデートが容易
- 開発初期はモノレポでも `go.mod` の `replace` ディレクティブで対応可能
- ARCHITECTURE.md の記述（「将来的に別リポジトリ化」）と整合

**影響**: ARCHITECTURE.md 付録のディレクトリ構造からの変更点。PROJECT_STRUCTURE.md sec 2 が最新の正とする。

---

### ADR-PS002: `cmd/myapp/` vs `cmd/server/`

| 項目 | 内容 |
|---|---|
| **ステータス** | 承認済み |
| **カテゴリ** | プロジェクト構造 |
| **関連ドキュメント** | PROJECT_STRUCTURE.md sec 3.1, ARCHITECTURE.md sec 3.2 |
| **優先度原則** | P2（AI 収束性） |

**状況**: ARCHITECTURE.md では `cmd/server/main.go`、タスク仕様では `cmd/myapp/main.go` が示されている。

**決定**: `cmd/myapp/main.go`（プロジェクト名と一致）を採用する。

**根拠**:
- `go build ./cmd/myapp` で生成されるバイナリ名が `myapp` になり、直感的
- 複数エントリーポイント（`cmd/myapp/`, `cmd/worker/` 等）を追加する余地を残す
- ARCHITECTURE.md の「Single Binary」設計思想に合致

---

### ADR-PS003: `db/migrations/` vs `migrations/`

| 項目 | 内容 |
|---|---|
| **ステータス** | 承認済み |
| **カテゴリ** | プロジェクト構造 |
| **関連ドキュメント** | PROJECT_STRUCTURE.md sec 3.5, DATA_ACCESS_DESIGN.md sec 4.2 |
| **優先度原則** | P2（AI 収束性） |

**状況**: ARCHITECTURE.md 付録では `migrations/`（ルート直下）、タスク仕様では `db/migrations/`。

**決定**: `db/migrations/` を採用する。

**根拠**:
- マイグレーションとシードデータを `db/` 配下に集約し、データベース関連ファイルの所在を明確化
- Rails の `db/migrate/` + `db/seeds.rb` パターンと整合
- プロジェクトルートのファイル数を減らし、見通しを向上

---

### ADR-PS004: `config/` ディレクトリの導入

| 項目 | 内容 |
|---|---|
| **ステータス** | 承認済み |
| **カテゴリ** | プロジェクト構造 |
| **関連ドキュメント** | PROJECT_STRUCTURE.md sec 3.2, SECURITY_DESIGN.md sec 2 |
| **優先度原則** | P2（AI 収束性） |

**状況**: アプリケーション固有の設定構造体の定義場所が不明確。

**決定**: `config/` ディレクトリを導入し、設定構造体・ルーティング設定・ミドルウェア設定を集約。

**根拠**:
- Django の `settings.py` + `urls.py` に相当するファイルの置き場所を明確にする
- `main.go` の肥大化を防ぐ
- `config/routes.go` でモジュール→URLプレフィックスのマッピングを一覧可能
- Go パッケージとして `config.MustLoad()` 等の型安全なAPIを提供

**影響**: SECURITY_DESIGN.md で `config/security.go` が追加され、この判断を拡張している。

---

### ADR-PS005: テンプレートの二重配置（共通 + モジュール固有）

| 項目 | 内容 |
|---|---|
| **ステータス** | 承認済み |
| **カテゴリ** | プロジェクト構造 |
| **関連ドキュメント** | PROJECT_STRUCTURE.md sec 3.6, DATA_ACCESS_DESIGN.md sec 6.2 |
| **優先度原則** | P2（AI 収束性） |

**状況**: テンプレートの一元化 vs モジュール内配置の選択。

**決定**: 共通テンプレートは `templates/`、モジュール固有は `modules/{name}/templates/{name}/` に配置。

**根拠**:
- 共通レイアウト（`base.html`）は全モジュールで共有するためルートレベルに配置が自然
- モジュール固有テンプレートはモジュールディレクトリ内に配置しモジュールの自己完結性を維持
- Django の `DIRS` + `APP_DIRS` パターンと同等の解決順序を提供
- `templates/{name}/` のサブディレクトリ名をモジュール名と一致させテンプレート名の衝突を防止

---

### ADR-PS006: `.broth/rules.md` のコミット対象

| 項目 | 内容 |
|---|---|
| **ステータス** | 承認済み |
| **カテゴリ** | DX（AI フレンドリー設計） |
| **関連ドキュメント** | PROJECT_STRUCTURE.md sec 6 |
| **優先度原則** | P2（AI 収束性） |

**決定**: `.broth/rules.md` は git 管理対象とする。

**根拠**:
- チーム全員（AIを含む）が同じルールを参照することが収束性の前提条件
- ルールの変更履歴を git で追跡できる
- `.broth/cache/` 等のツール生成ファイルのみを `.gitignore` で除外

---

### ADR-PS007: Makefile の採用

| 項目 | 内容 |
|---|---|
| **ステータス** | 承認済み |
| **カテゴリ** | DX |
| **関連ドキュメント** | PROJECT_STRUCTURE.md sec 3.9 |
| **優先度原則** | P3（チーム運用オーバーヘッド最小化） |

**決定**: タスクランナーとして `Makefile` を採用する。

**根拠**:
- Go プロジェクトで最も一般的なタスクランナーであり、追加依存がない
- `make serve`, `make test`, `make migrate-up` 等の短縮コマンドを提供
- 使い分け: Makefile は開発者向けショートカット、`broth` CLI はフレームワーク機能

---

## 7. 並列処理・ジョブ（ADR-C001〜C005）

**出典**: [CONCURRENCY_DESIGN.md](./CONCURRENCY_DESIGN.md) sec 10

### ADR-C001: インメモリジョブと永続ジョブの2層構造

| 項目 | 内容 |
|---|---|
| **ステータス** | 承認済み |
| **カテゴリ** | 並列処理・ジョブ |
| **関連ドキュメント** | CONCURRENCY_DESIGN.md sec 1, sec 4, sec 5 |
| **優先度原則** | P1（Go イディオム）, P3（チーム運用） |

**状況**: バックグラウンドジョブを全てDB永続化 / 全てインメモリ / 2層構造の3選択肢。

**決定**: **2層構造**（インメモリ + DB永続化）を採用。

**根拠**:
- ウェルカムメール送信（失敗しても再送可能）にDBオーバーヘッドは不要
- 決済処理（失敗時リトライ必須）にはDB永続化が必要
- Go のチャネルによるインメモリキューは極めて軽量（ns単位）
- 開発者は `job.WithPersist()` オプションで明示的に選択する

---

### ADR-C002: DB ポーリング vs LISTEN/NOTIFY

| 項目 | 内容 |
|---|---|
| **ステータス** | 承認済み（Phase 1） |
| **カテゴリ** | 並列処理・ジョブ |
| **関連ドキュメント** | CONCURRENCY_DESIGN.md sec 5.5 |
| **優先度原則** | P4（将来拡張性 / YAGNI） |

**状況**: 永続ジョブの取得方法として、ポーリング / LISTEN/NOTIFY / 併用の3選択肢。

**決定**: **ポーリング**（`SELECT FOR UPDATE SKIP LOCKED`）を Phase 1 で採用。Phase 2 で LISTEN/NOTIFY への最適化を検討。

**根拠**:
- ポーリング間隔1秒はほとんどのユースケースで十分
- `SELECT FOR UPDATE SKIP LOCKED` は PostgreSQL のネイティブ機能で高効率
- 将来的な MySQL / Redis 対応時にポーリングモデルの方が移植しやすい
- LISTEN/NOTIFY は最適化として後から追加可能

---

### ADR-C003: ジョブのシリアライズ方式

| 項目 | 内容 |
|---|---|
| **ステータス** | 承認済み |
| **カテゴリ** | 並列処理・ジョブ |
| **関連ドキュメント** | CONCURRENCY_DESIGN.md sec 5.2 |
| **優先度原則** | P1（Go イディオム） |

**状況**: ジョブペイロードのシリアライズ方式を決定する必要がある。

**決定**: JSON（`encoding/json`）を使用し、PostgreSQL の JSONB カラムに格納。

**根拠**:
- JSONB カラムによりDB上での検索・フィルタリングが可能
- 管理画面でのペイロード表示がヒューマンリーダブル
- `encoding/json` は標準ライブラリであり追加依存なし
- パフォーマンスが問題になった場合に `encoding/gob` や Protocol Buffers への移行パスがある

---

### ADR-C004: WebSocket ライブラリの選定

| 項目 | 内容 |
|---|---|
| **ステータス** | 承認済み |
| **カテゴリ** | 並列処理・リアルタイム |
| **関連ドキュメント** | CONCURRENCY_DESIGN.md sec 7 |
| **優先度原則** | P1（Go イディオム） |

**状況**: `golang.org/x/net/websocket` / `nhooyr.io/websocket` / `gorilla/websocket` の3選択肢。

**決定**: **`nhooyr.io/websocket`** を推奨する。ただしフレームワークコアには含めず、アプリケーション側の依存とする。

**根拠**:
- `context.Context` との統合が設計段階から組み込まれている
- 依存が最小限（Broth の設計思想に合致）
- WebSocket はアプリケーション固有の機能であり、フレームワークコアに含めるべきではない
- `broth/ws` パッケージは Hub パターンのヘルパーのみ提供

---

### ADR-C005: スケジューラのリーダー選出方式

| 項目 | 内容 |
|---|---|
| **ステータス** | 承認済み（Phase 1） |
| **カテゴリ** | 並列処理・スケジューラ |
| **関連ドキュメント** | CONCURRENCY_DESIGN.md sec 6.5 |
| **優先度原則** | P3（チーム運用）, P4（将来拡張性） |

**状況**: 複数インスタンスでの重複実行防止方式として、DB行ロック / Redis / etcd / 運用規約の4選択肢。

**決定**: **DB 行ロック** を Phase 1 で採用。

**根拠**:
- 「最小構成: 単一バイナリ + DB」の設計思想に合致
- 定期実行の粒度（分単位）ではDBロックのレイテンシは問題にならない
- 単一インスタンス構成ではリーダー選出自体が不要
- Redis 導入時に Redis 分散ロックへの移行パスがある

---

## 8. セキュリティ（ADR-SEC001〜SEC006）

**出典**: [SECURITY_DESIGN.md](./SECURITY_DESIGN.md) sec 9

### ADR-SEC001: コンテキスト自動適用の導入

| 項目 | 内容 |
|---|---|
| **ステータス** | 承認済み |
| **カテゴリ** | セキュリティ |
| **関連ドキュメント** | SECURITY_DESIGN.md sec 3, ARCHITECTURE.md ADR-001 |
| **優先度原則** | P3（チーム運用 / Secure by Default） |

**状況**: SSR と API で適用すべきセキュリティスタックが異なる。既存フレームワークはルートグループで手動分離する。

**決定**: リクエスト単位でSSR/APIを自動判定し、適切なセキュリティスタックを自動適用する。

**根拠**:
- ルートグループの手動分離は設定ミスの温床であり、Secure by Default 原則に反する
- 判定ロジックは5つのHTTPヘッダの静的チェックであり、予測可能かつデバッグ可能
- デフォルトをSSR（安全側）に倒すことで、誤判定時もセキュリティが維持
- ルート単位のオーバーライド（`ForceContext`）で明示的制御が可能

**リスクと緩和策**:
- 開発モードでのヘッダ出力（`X-Broth-Context`）、`broth routes` での一覧表示、ドキュメント充実で透明性を担保

**影響**: ADR-001 の `context.Context` パターンに準拠。`requestContextKey` 型でコンテキストに格納。

---

### ADR-SEC002: CSRF に Synchronizer Token パターンを採用

| 項目 | 内容 |
|---|---|
| **ステータス** | 承認済み |
| **カテゴリ** | セキュリティ |
| **関連ドキュメント** | SECURITY_DESIGN.md sec 6.1, DATA_ACCESS_DESIGN.md sec 6 |
| **優先度原則** | P3（Secure by Default） |

**状況**: Synchronizer Token vs ダブルサブミットCookie の選択。

**決定**: Synchronizer Token パターンを採用する。

**根拠**:
- Broth は `broth/session` によるセッション基盤を標準搭載しており追加コストが低い
- サブドメインからの Cookie 操作によるバイパス攻撃に対してより堅牢
- Django と同じパターンであり、成熟した設計を踏襲

**影響**: `broth/render` の `csrfField` テンプレート関数、テンプレートの `{{csrfField .Request}}` パターンと統合。

---

### ADR-SEC003: パスワードハッシュのデフォルトに bcrypt を採用

| 項目 | 内容 |
|---|---|
| **ステータス** | 承認済み |
| **カテゴリ** | セキュリティ |
| **関連ドキュメント** | SECURITY_DESIGN.md sec 4.5 |
| **優先度原則** | P1（Go イディオム / 最小依存） |

**状況**: bcrypt / argon2id / scrypt の選択。

**決定**: bcrypt をデフォルトとし、argon2id はオプションとして提供。

**根拠**:
- `golang.org/x/crypto/bcrypt` は Go 準標準ライブラリで追加依存が最小
- bcrypt は20年以上の実績があり、既知の脆弱性がない
- argon2id はより理論的に安全だがパラメータ調整が複雑
- ARCHITECTURE.md の「サードパーティ依存は最小限」原則に合致

---

### ADR-SEC004: レート制限に Token Bucket を採用

| 項目 | 内容 |
|---|---|
| **ステータス** | 承認済み |
| **カテゴリ** | セキュリティ |
| **関連ドキュメント** | SECURITY_DESIGN.md sec 6.5 |
| **優先度原則** | P1（Go イディオム / 標準ライブラリ活用） |

**状況**: Token Bucket / Sliding Window / Fixed Window の選択。

**決定**: Token Bucket アルゴリズムを採用。

**根拠**:
- `golang.org/x/time/rate` で Go 準標準として提供
- バースト許容と平均レート制限のバランスが良い
- 実装が単純で、インメモリ / DB / Redis の各バックエンドに移植しやすい

---

### ADR-SEC005: セキュリティヘッダのデフォルト有効化

| 項目 | 内容 |
|---|---|
| **ステータス** | 承認済み |
| **カテゴリ** | セキュリティ |
| **関連ドキュメント** | SECURITY_DESIGN.md sec 8 |
| **優先度原則** | P3（Secure by Default） |

**状況**: セキュリティヘッダをデフォルト有効にするか、オプトインにするか。

**決定**: デフォルトで有効にする。オプトアウトは個別のヘッダ単位で可能。

**根拠**:
- Secure by Default 原則に合致
- 「知らなかったから設定しなかった」という事態を防止
- 個別オーバーライドが可能なため、特殊なケースにも対応
- Django の `SecurityMiddleware`、Laravel のデフォルトヘッダと同じアプローチ

**設定されるヘッダ**: `X-Content-Type-Options`, `X-Frame-Options`, `X-XSS-Protection`, `Referrer-Policy`, `Content-Security-Policy`, `Strict-Transport-Security`, `Permissions-Policy`

---

### ADR-SEC006: JWT に `golang-jwt/jwt` を採用

| 項目 | 内容 |
|---|---|
| **ステータス** | 承認済み（改訂） |
| **カテゴリ** | セキュリティ |
| **関連ドキュメント** | SECURITY_DESIGN.md sec 4.3 |
| **優先度原則** | P1（Go イディオム / 最小依存） |

**状況**: JWT の生成・検証に外部ライブラリ（`golang-jwt/jwt` 等）を使うか、自前実装するか。

**決定**: `github.com/golang-jwt/jwt/v5` を採用する。

**根拠**:
- JWT の自前実装はタイミング攻撃対策、アルゴリズム混同攻撃（alg=none 問題）、ヘッダインジェクション等のセキュリティ上のエッジケースが多く、安全な実装が困難
- `golang-jwt/jwt` は Go エコシステムにおける事実上の標準であり、広くセキュリティレビューを受けている
- セキュリティに関わる実績あるライブラリは「サードパーティ依存は最小限」原則の例外として許容する
- RSA/ECDSA への将来的な拡張もライブラリが標準でサポート

**影響**: ARCHITECTURE.md の P1 原則に「セキュリティライブラリは例外として許容」の補足を追加。

**変更履歴**: 初期設計では HMAC-SHA256 自前実装としていたが、セキュリティ上の懸念から改訂

---

## 9. データアクセス（ADR-D001〜D007）

**出典**: [DATA_ACCESS_DESIGN.md](./DATA_ACCESS_DESIGN.md) 付録

### ADR-D001: Bob を推奨データアクセスライブラリとして採用

| 項目 | 内容 |
|---|---|
| **ステータス** | 改訂（Bob 採用に変更） |
| **カテゴリ** | データアクセス |
| **関連ドキュメント** | DATA_ACCESS_DESIGN.md sec 2, ARCHITECTURE.md sec 3.4 |
| **優先度原則** | P1（Go イディオム） |

**状況**: Go にはGORM, ent, Bun, sqlc, SQLBoiler, Bob 等のデータアクセスライブラリがある。

**決定**: Bob（`github.com/stephenafamo/bob`）を推奨データアクセスライブラリとして採用する。`broth/db` は Bob を薄くラップし、`broth generate model` は内部で `bobgen-psql` を呼び出す。

**根拠**:
- Bob は SQLBoiler のメンテナーが設計し直した後継であり、database-first・コード生成・型安全・`interface{}` 不使用を全て満たす
- GORM はリフレクション多用・`interface{}` 氾濫が Broth の設計原則と根本的に衝突（ADR-D007 参照）
- sqlc は設計原則との整合は高いが、JOIN → フラット構造体のマッピングコストが実務で重く、リレーション操作の生産性が不足
- SQLBoiler は思想的に近いがメンテナンス体制が不安定で、後継の Bob が存在する
- Bob のテスト用ファクトリー生成（factory_bot インスパイア）は Broth のテスト戦略に直接貢献
- `bob.Executor` インターフェースが `*sql.DB` / `*sql.Tx` を受け入れるため、`broth/db` の `ConnFromContext` との統合が自然

**影響**: ADR-003 のリフレクション方針と整合（Bob はコード生成ベースでリフレクション不使用）。`broth generate model` コマンドで Bob ベースのコード生成を提供。

**変更履歴**: 初期設計では「ORM 不採用、database/sql + コード生成（sqlc 推奨）」としていたが、リレーション操作の生産性と設計原則との整合を総合評価し、Bob 採用に変更。

---

### ADR-D002: マイグレーションに goose を採用

| 項目 | 内容 |
|---|---|
| **ステータス** | 承認済み |
| **カテゴリ** | データアクセス |
| **関連ドキュメント** | DATA_ACCESS_DESIGN.md sec 4.1, PROJECT_STRUCTURE.md ADR-PS003 |
| **優先度原則** | P1（Go イディオム / 最小依存） |

**状況**: マイグレーションツールとして goose / Atlas / golang-migrate / 自前実装の選択。

**決定**: goose を `broth/migrate` パッケージで内部ラップして使用する。

**根拠**:
- SQL ファイルベースで Broth の「SQL を第一級市民にする」方針と合致
- Go の `embed` パッケージとの親和性が高い（本番バイナリ埋め込み）
- 自前実装はバージョン管理・ロック・競合制御の複雑さを引き受ける必要がありROIが合わない
- Atlas は高機能だが HCL ベースの宣言的アプローチが Broth と合わない

---

### ADR-D003: Admin 画面はコード生成ベース

| 項目 | 内容 |
|---|---|
| **ステータス** | 承認済み |
| **カテゴリ** | データアクセス・管理画面 |
| **関連ドキュメント** | DATA_ACCESS_DESIGN.md sec 5, ARCHITECTURE.md sec 4 |
| **優先度原則** | P1（Go イディオム） |

**状況**: Django Admin 相当の機能を Go で実現する方法。

**決定**: `broth admin generate` コマンドで Go ハンドラ + HTML テンプレートを生成する。

**根拠**:
- Django Admin のリフレクションベース動的生成は Go のコンパイル時安全性哲学に反する
- コード生成であれば生成後に開発者が自由にカスタマイズ可能
- 生成コードは Go コンパイラが型チェックするため安全性が保証される
- 初期コストは Django Admin より高いが、カスタマイズ時の透明性で勝る

**影響**: ADR-003 のリフレクション許可範囲に `broth/admin` が含まれるが、実際のAdmin画面はコード生成。リフレクションは struct tag の解析（`broth generate` 時のAST解析）にのみ使用。

---

### ADR-D004: テンプレートエンジンは html/template 拡張

| 項目 | 内容 |
|---|---|
| **ステータス** | 承認済み |
| **カテゴリ** | データアクセス・テンプレート |
| **関連ドキュメント** | DATA_ACCESS_DESIGN.md sec 6, PROJECT_STRUCTURE.md ADR-PS005 |
| **優先度原則** | P1（Go イディオム / 最小依存） |

**状況**: テンプレートエンジンの選定。plush, jet 等の外部テンプレートエンジンの選択肢。

**決定**: Go 標準の `html/template` を拡張する。外部テンプレートエンジンは採用しない。

**根拠**:
- `html/template` は自動エスケープ付きで XSS 対策が標準
- サードパーティ依存を最小限にする方針に合致
- レイアウト継承は `template.ParseFiles` の組み合わせと命名規約で実現可能
- FuncMap による拡張で十分な表現力を確保

**影響**: SECURITY_DESIGN.md の XSS 対策（sec 6.2）が `html/template` の自動エスケープに依存。

---

### ADR-D005: broth/form でのリフレクション使用

| 項目 | 内容 |
|---|---|
| **ステータス** | 承認済み |
| **カテゴリ** | データアクセス・フォーム |
| **関連ドキュメント** | DATA_ACCESS_DESIGN.md sec 6.7, ARCHITECTURE.md ADR-003 |
| **優先度原則** | P1（Go イディオム） |

**状況**: フォームバインディングでリフレクションを使うか、コード生成で代替するか。

**決定**: `broth/form` でのリフレクション使用を許可する（ADR-003 で明示的に許可された範囲）。

**根拠**:
- フォームバインディングは `encoding/json` と同じパターンであり、Go コミュニティで広く受容
- コード生成で代替するとフォーム定義のたびに `go generate` が必要で開発体験が低下
- リフレクションの使用範囲を `Bind()` と `Validate()` の2関数に限定し、影響範囲を最小化

---

### ADR-D006: `broth generate model` は bobgen-psql をラップ

| 項目 | 内容 |
|---|---|
| **ステータス** | 改訂（Bob 採用に伴い変更） |
| **カテゴリ** | データアクセス・コード生成 |
| **関連ドキュメント** | DATA_ACCESS_DESIGN.md sec 2.4, ADR-D001, ADR-D002 |
| **優先度原則** | P4（将来拡張性 / YAGNI） |

**状況**: `broth generate model` のコード生成エンジンを自前実装するか、既存ツールをラップするか。

**決定**: `broth generate model` は内部で `bobgen-psql` を呼び出し、DB スキーマからモデルコード・テスト用ファクトリーを生成する。複雑な手書き SQL クエリには `bobgen-sql` を優先的に使用し、不足する場合は sqlc を補助的に利用可能とする。

**根拠**:
- ADR-D001（Bob 採用）に伴い、コード生成エンジンも Bob のツールチェインで統一
- ADR-D002（goose ラップ）と同じ「既存ツールのラップ/連携」路線で一貫性がある
- `bobgen-psql` は DB スキーマからモデル・リレーション・ファクトリーを一括生成でき、sqlc より生産性が高い
- sqlc は Bob で表現しにくい複雑なクエリの補助として位置づけ、排他的な選択ではなく併用可能

**影響**: Bob がビルド時の外部ツール依存に加わる。

**変更履歴**: 初期設計では sqlc ラッパーとしていたが、Bob 採用決定（ADR-D001 改訂）に伴い変更。

---

### ADR-D007: GORM 不採用

| 項目 | 内容 |
|---|---|
| **ステータス** | 承認済み |
| **カテゴリ** | データアクセス |
| **関連ドキュメント** | DATA_ACCESS_DESIGN.md sec 2.2, ADR-D001 |
| **優先度原則** | P1（Go イディオム） |

**状況**: GORM は Go で最も人気のある ORM（GitHub Stars 37k+）であり、採用候補として検討した。

**決定**: GORM を採用しない。

**根拠**:
1. **`interface{}` の氾濫**: 全公開メソッド（`Create()`, `Find()`, `Where()` 等）が `interface{}` を受け入れ、Broth の型安全原則（ARCHITECTURE.md 制約5）と根本的に衝突
2. **リフレクション多用**: 構造体→SQL変換がランタイムリフレクションに依存し、ADR-003 のリフレクション最小化方針と衝突。パフォーマンスも `database/sql` 直接利用の5-6倍遅い
3. **暗黙の振る舞い**: ソフトデリート（`DeletedAt` フィールド自動検出）、自動タイムスタンプ等の暗黙動作が Go の明示性文化と衝突
4. **デバッグ困難性**: リフレクションベースの SQL 生成は発行 SQL の予測が困難で、N+1 問題等のパフォーマンス問題が暗黙的に発生しうる

**影響**: Bob 採用（ADR-D001）の根拠の一部を構成する。GORM からの移行ガイドをドキュメントで提供予定。

---

## 10. クロスレビュー記録

6つの設計書間の整合性を4つの視点から横断的に検証した結果を記録する。

### 10.1 コアアーキテクチャ ⇔ プロジェクト構造・DX

| 検証項目 | 結果 | 詳細 |
|---|---|---|
| レイヤードアーキテクチャとディレクトリ構造の一致 | **整合** | ARCHITECTURE.md の4層がPROJECT_STRUCTURE.md sec 3.3 のファイル配置と完全対応 |
| `broth/` ディレクトリの扱い | **差異あり（解決済み）** | ARCHITECTURE.md 付録では `broth/` をプロジェクト内に配置。ADR-PS001 で Go モジュール外部依存に変更。PROJECT_STRUCTURE.md が最新の正 |
| `cmd/server/` vs `cmd/myapp/` | **差異あり（解決済み）** | ARCHITECTURE.md では `cmd/server/`、ADR-PS002 で `cmd/myapp/` に統一。PROJECT_STRUCTURE.md が最新の正 |
| `migrations/` vs `db/migrations/` | **差異あり（解決済み）** | ARCHITECTURE.md 付録では `migrations/`、ADR-PS003 で `db/migrations/` に統一 |
| DI パターン（ADR-002）と main.go のワイヤリング | **整合** | コンストラクタ注入パターンが PROJECT_STRUCTURE.md sec 3.1 の main.go コード例に一貫 |

### 10.2 コアアーキテクチャ ⇔ 並列処理・ジョブ

| 検証項目 | 結果 | 詳細 |
|---|---|---|
| レイヤー配置の整合性 | **整合** | ジョブ投入（`job.Enqueue`）はアプリケーションレイヤー、ジョブ実行基盤は横断的関心事として配置（CONCURRENCY_DESIGN.md sec 2） |
| context.Context の伝播パターン | **整合** | ADR-001 の context パターンがジョブ内でも継承。TraceID がジョブ間で伝播（CONCURRENCY_DESIGN.md 付録B） |
| Single Binary 思想との整合 | **整合** | HTTP サーバー + ジョブワーカー + スケジューラが単一バイナリに内蔵（CONCURRENCY_DESIGN.md sec 8.1） |
| graceful shutdown の統合 | **整合** | 停止順序（HTTP → スケジューラ → インメモリワーカー → 永続ワーカー → モジュール → DB）が明確に定義（CONCURRENCY_DESIGN.md sec 9） |

### 10.3 セキュリティ ⇔ プロジェクト構造・DX

| 検証項目 | 結果 | 詳細 |
|---|---|---|
| config/ ディレクトリへの統合 | **整合** | SECURITY_DESIGN.md が `config/security.go` を追加し、ADR-PS004 の config/ ディレクトリ構造を拡張 |
| middleware 設定の整合 | **整合** | SECURITY_DESIGN.md の `config/middleware.go` が PROJECT_STRUCTURE.md sec 3.2 のミドルウェアチェーンを拡張（SecurityHeaders, ContextDetect, RateLimit を追加） |
| .env / .env.example の拡張 | **整合** | SECRET_KEY 等のセキュリティ設定が .env.example に追加（SECURITY_DESIGN.md sec 7.2） |
| テンプレートへの CSRF 統合 | **整合** | `{{csrfField}}` テンプレート関数が ADR-PS005 のテンプレート二重配置構造で機能 |
| AI ルール（.broth/rules.md）へのセキュリティルール反映 | **要確認** | `.broth/rules.md` の「禁止事項」にセキュリティ関連ルールを追記すべき（model.go に database/sql を import しない等は記載済み。セキュリティ固有のルールは未記載） |

### 10.4 データアクセス ⇔ コアアーキテクチャ

| 検証項目 | 結果 | 詳細 |
|---|---|---|
| Repository パターンの一致 | **整合** | ARCHITECTURE.md sec 3.4 / MODULE_DESIGN.md sec 2 / DATA_ACCESS_DESIGN.md sec 1.5 で同一パターン |
| TxManager のコンテキスト伝搬 | **整合** | ADR-001 の context パターンに基づき `ConnFromContext` でトランザクションを透過的に伝搬 |
| コード生成 vs リフレクションの一貫性 | **整合** | ADR-003 / ADR-D001 / ADR-D003 / ADR-D005 が整合。ORM不使用、コード生成優先、form のみリフレクション許可 |
| マイグレーションファイルの配置 | **整合** | ADR-PS003（`db/migrations/`）と DATA_ACCESS_DESIGN.md sec 4.2 が一致 |
| Admin 画面とモジュール構造 | **整合** | `broth admin generate` がモジュール内に `admin_handler.go` / `admin_routes.go` を生成し、MODULE_DESIGN.md の構造を拡張 |

---

## 11. 将来検討事項（Future / Pending）

各設計書で「Phase 2以降」「将来的に」と記載された項目を集約する。

### Phase 2 予定

| 項目 | 出典 | 概要 |
|---|---|---|
| イベントベースのモジュール間通信 | MODULE_DESIGN.md ADR-M002 | 直接参照に加え、通知・監査ログ等でイベント駆動を部分導入 |
| LISTEN/NOTIFY 対応 | CONCURRENCY_DESIGN.md ADR-C002 | DBポーリングに加え、PostgreSQL LISTEN/NOTIFY でジョブ取得を最適化 |
| Redis バックエンド | CONCURRENCY_DESIGN.md sec 8.2 | セッション / ジョブキュー / WebSocket ブロードキャスト / キャッシュ / レート制限の Redis 対応 |
| ジョブチェーン / ワークフロー | CONCURRENCY_DESIGN.md sec 5.10 | Celery Canvas 相当のジョブ連鎖機能 |
| 結果バックエンド | CONCURRENCY_DESIGN.md sec 5.10 | ジョブの戻り値を後から取得する機能 |
| Admin フィルタ + 検索 | DATA_ACCESS_DESIGN.md sec 5.7 | struct tag の `filter` / `search` に基づくフィルタリングと全文検索 |
| broth generate model 拡張 | DATA_ACCESS_DESIGN.md sec 2.4, ADR-D006 | Phase 1 は bobgen-psql ラッパー。Phase 2 で bobgen-sql 検証・カスタムクエリ対応 |
| argon2id 対応 | SECURITY_DESIGN.md ADR-SEC003 | bcrypt に加え argon2id をオプション提供 |
| OAuth2 プロバイダ連携 | SECURITY_DESIGN.md sec 4.4 | Google / GitHub 等の OAuth2 プロバイダ統合 |

### Phase 3 以降

| 項目 | 出典 | 概要 |
|---|---|---|
| Wire 等のコード生成DI | ARCHITECTURE.md ADR-002, PROJECT_STRUCTURE.md sec 7.1 | モジュール9個以上で依存グラフが複雑化した場合に検討 |
| Admin ダッシュボード | DATA_ACCESS_DESIGN.md sec 5.7 | Admin トップページの統計情報表示、カスタムウィジェット |
| Admin 権限管理 | DATA_ACCESS_DESIGN.md sec 5.7 | ロールベースの Admin アクセス制御、監査ログ |
| クエリビルダ拡張 | DATA_ACCESS_DESIGN.md sec 2.7 | WHERE 句以外の動的組み立て（JOIN, GROUP BY 等） |
| パフォーマンス計装 | DATA_ACCESS_DESIGN.md sec 7 | スロークエリ自動検出、クエリ実行計画の可視化 |
| Apps 概念の導入 | ADR-M006 | Module 数 9 以上・チーム 15 人以上・3+ ビジネスドメイン時に `apps/` グルーピングを再検討 |

### Phase 2 予定（追加）

| 項目 | 出典 | 概要 |
|---|---|---|
| 主要ドキュメントの英語化 | ADR-0004 | README、主要設計書、API ドキュメントの英語版を整備。国際的なコントリビューションを促進する |
| MySQL 対応 | CONCURRENCY_DESIGN.md ADR-C002 | `SELECT FOR UPDATE SKIP LOCKED`（MySQL 8.0+ 対応）への移行パスを実装。`LISTEN/NOTIFY` の代替として MySQL 向けポーリング最適化 |

### 未解決の設計課題

| 項目 | 詳細 |
|---|---|
| `.broth/rules.md` へのセキュリティルール追記 | クロスレビュー 10.3 で検出。CSRF / 認証 / 認可に関する AI 向けルールの追加が必要 |
| ARCHITECTURE.md 付録の更新 | ADR-PS001〜PS003 による変更（`broth/` 廃止、`cmd/myapp/`、`db/migrations/`）を ARCHITECTURE.md 付録のディレクトリ構造に反映すべき |

---

## 12. 差別化機能サマリ

Broth フレームワークが他の Go フレームワークおよび Django/Rails と比較して独自に提供する機能を集約する。

### Broth 固有の差別化機能

| 機能 | 概要 | 関連ADR | 競合比較 |
|---|---|---|---|
| **SSR/API コンテキスト自動適用** | リクエスト単位でSSR/APIを自動判定し、CSRF/認証/エラーレスポンスを自動切替 | ADR-SEC001 | 他の Go FW / Django / Rails にない独自機能 |
| **service.go の規約化** | ビジネスロジックの唯一の公式置き場所を明示的に定義 | ADR-004 | Django/Rails は暗黙的。Go FW は規約なし |
| **Single Binary デプロイ** | HTTP + ジョブワーカー + スケジューラを単一バイナリに内蔵。最小構成は1バイナリ + DB | ADR-C001, C005 | Django は4コンポーネント構成（Django + Celery Worker + Beat + Redis） |
| **インメモリ + DB 2層ジョブ** | goroutine ベースの軽量ジョブと DB 永続ジョブの使い分け | ADR-C001 | Celery/Sidekiq は常にブローカー経由 |
| **コード生成 Admin 画面** | Django Admin のコード生成版。コンパイル時型安全、カスタマイズ透明性 | ADR-D003 | Django Admin は実行時リフレクション |
| **AI フレンドリー設計** | `.broth/rules.md` でAI向けルールを明文化、`broth rules export` で各AI形式に変換 | ADR-PS006 | 他FWにない概念 |
| **コンパイル時境界チェック** | Go の `internal/` + depguard + カスタム go vet でレイヤー違反をコンパイル/lint 時に検出 | ADR-M001, ADR-005 | Django/Rails は規約ベース（弱い強制力） |
| **Secure by Default** | セキュリティヘッダ・CSRF・レート制限がデフォルト有効。SECRET_KEY 未設定で起動拒否 | ADR-SEC005 | Django に近いが、SSR/API 自動切替が上位互換 |

### 設計方針による差別化

| 方針 | 効果 | 関連ADR |
|---|---|---|
| リフレクション最小化 + コード生成優先 | コンパイル時安全性の最大化、IDE サポートの向上 | ADR-003, ADR-D001, ADR-D003, ADR-D007 |
| Bob（database-first コード生成）採用 | 型安全な Query Mod、Eager Loading、テストファクトリーにより生産性と安全性を両立。GORM の暗黙動作を排除 | ADR-D001, ADR-D007 |
| `context.Context` 標準パターン遵守 | `net/http` エコシステムとの完全互換性 | ADR-001 |
| Go 準標準ライブラリの最大活用 | `golang.org/x/crypto/bcrypt`, `golang.org/x/time/rate`, `golang.org/x/sync/errgroup` 等でサードパーティ依存を最小化。セキュリティライブラリ（`golang-jwt/jwt`）とデータアクセス（Bob）は実績・設計原則との整合から例外的に許容 | ADR-SEC003, ADR-SEC004, ADR-SEC006, ADR-D001 |

---

## 13. 競合分析サマリ

> 以下は主要な Go Web フレームワークとの比較分析サマリである。

### Buffalo の失敗原因と Broth の回避策

| Buffalo の失敗パターン | Broth の対応 ADR |
|---|---|
| 独自 Context 型（`buffalo.Context`）→ `net/http.Handler` 非互換 | **ADR-001**: `context.Context` + ジェネリクス。`net/http.Handler` 完全互換 |
| Pop ORM（リフレクション依存の ActiveRecord 模倣） | **ADR-D001**: Bob 採用（database-first コード生成、リフレクション不使用） |
| Plush テンプレートエンジン（`html/template` 非互換） | **ADR-D004**: `html/template` 拡張。外部テンプレートエンジン不使用 |
| gorilla toolkit への重度依存 → アーカイブ化で致命傷 | **ARCHITECTURE.md P1**: サードパーティ依存最小限 |
| シングルメンテナ依存（Mark Bates） | 要検討: ガバナンスモデルの明文化 |
| Webpack 統合 → Node.js 依存 | **Single Binary**: CGO 不要、`go build` 一発 |

### Beego の失敗原因と Broth の回避策

| Beego の失敗パターン | Broth の対応 ADR |
|---|---|
| コントローラの「継承」パターン（`beego.Controller` 埋め込み） | **ADR-001**: 独自 Context 型を作らない |
| `interface{}` の氾濫（`c.Data` マップ等） | **ARCHITECTURE.md 制約5**: `interface{}` を公開 API に使わない |
| グローバル状態（`beego.Run()` 等） | **ADR-002**: DI コンテナ不使用。コンストラクタ注入 |
| `init()` によるグローバルモデル登録 | **ADR-M001**: `broth.Module` インターフェースの型安全な登録 |
| リフレクション ORM（`beego/orm`） | **ADR-D001, ADR-003, ADR-D007**: Bob 採用（コード生成ベース、リフレクション不使用）、GORM 明示的不採用 |

### エコシステム内の Broth のポジション

Broth は「薄いルーター（Chi/Echo/Gin）」と「Go イディオムを逸脱したフルスタック（Buffalo/Beego）」の間にあるポジション -- **Go イディオムに忠実なフルスタック** -- を占める。このポジションは2026年時点で空白地帯であり、Broth の差別化ポイントとなる。

---

## 14. バージョニング方針

### ADR-VER001: Go Modules セマンティックバージョニングに準拠

| 項目 | 内容 |
|---|---|
| **ステータス** | 承認済み |
| **カテゴリ** | プロジェクト運営 |
| **優先度原則** | P1（Go イディオム） |

**状況**: フレームワークのバージョニングと後方互換性の方針を定める必要がある。

**決定**: Go Modules のセマンティックバージョニング（semver）に従う。

**方針**:
- **v0.x.x**: API 安定性を保証しない。破壊的変更は CHANGELOG に明記し、マイナーバージョンで実施
- **v1.0.0**: 公開 API の安定性を保証。破壊的変更はメジャーバージョンアップのみで実施
- **v1.0.0 到達条件**: Phase 1 の全機能が実装され、少なくとも1つの実プロジェクトで実証されていること
- **非推奨 API**: `// Deprecated:` コメント + 1メジャーバージョンの猶予期間を設ける

**根拠**:
- Go Modules はモジュールパスにメジャーバージョンを含める設計（`v2`、`v3`）であり、semver 遵守が Go エコシステムの前提
- v0.x.x で安定性を保証しないことで、初期の設計変更に柔軟に対応できる
- Buffalo の教訓: v1.0.0 を出す前に停滞したため、「安定版なし」の状態がユーザーの信頼を損なった
