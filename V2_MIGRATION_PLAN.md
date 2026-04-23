# Polymarket CLOB V2 Migration Plan

> **Target cutover:** April 28, 2026 (~11:00 UTC)  
> **Test endpoint:** `https://clob-v2.polymarket.com`  
> **Post-cutover endpoint:** `https://clob.polymarket.com` (auto-switches to V2)  
> **Branch strategy:** `main` → feature branch `feat/v2-clob` → merge after live validation

---

## 1. Executive Summary

Polymarket is shipping a coordinated V2 upgrade: new Exchange contracts, rewritten CLOB backend, and a new collateral token (pUSD). This plan details every code change required in `polymarket-go-sdk` to support V2, plus a live-API validation strategy so we know it works before cutover day.

### Critical V2 Changes
| Area | V1 | V2 |
|------|-----|-----|
| **CLOB host (pre-cutover)** | `clob.polymarket.com` | `clob-v2.polymarket.com` |
| **CLOB host (post-cutover)** | `clob.polymarket.com` (switches to V2 automatically) | `clob.polymarket.com` |
| **Exchange domain ver.** | `"1"` | `"2"` |
| **Verifying contract** | `0x4bFb41d5B3570DeFd03C39a9A4D8dE6Bd8B8982E` | `0xE111180000d2663C0091e4f400237545B87B996B` |
| **NegRisk contract** | `0xC5d563A36AE78145C45a50134d48A1215220f80a` | `0xe2222d279d744050d28e00520010520000310F59` |
| **Order uniqueness** | `nonce` (uint256) | `timestamp` (ms since epoch) |
| **Fees** | Embedded in signed order (`feeRateBps`) | Operator-set at match time; makers never pay |
| **Builder attribution** | HMAC headers (`POLY_BUILDER_*`) + `builder-signing-sdk` | `builderCode` (bytes32) on the order itself |
| **Signed struct fields** | `salt, maker, signer, taker, tokenId, makerAmount, takerAmount, expiration, nonce, feeRateBps, side, signatureType` | Same **minus** `taker, expiration, nonce, feeRateBps` **plus** `timestamp, metadata, builder` |
| **POST body** | V1 fields | V1 zeroed fields **+** `timestamp`, `metadata`, `builder` |
| **Collateral** | USDC.e | pUSD |
| **Markets pagination** | `GET /markets` with `cursor`/`limit` | `GET /markets/keyset` recommended (cursor-based, `"markets"` wrapper key). Old endpoint deprecated but still works. |
| **`closed` default** | `closed` defaults to `true` (includes closed markets) | `closed` defaults to `false` (Apr 9, 2026) |
| **Fee info source** | `/fee-rate` endpoint | `feeSchedule` object inside market response (Mar 31, 2026) |

---

## 2. Current State Assessment

### What already works well
- **Transport layer** (`pkg/transport`) already supports custom base URLs, auth injection, retries, and circuit breaking. Switching hosts is a one-line config change.
- **Defensive JSON unmarshaling** exists in several places (`OrderResponse.UnmarshalJSON`, `PricesHistoryResponse.UnmarshalJSON`, `healthResponse.UnmarshalJSON`). This pattern should be extended to new V2 response shapes.
- **Client interface immutability** (`WithAuth`, `WithBuilderConfig`, etc.) returns cloned instances. We can add `WithCLOBVersion` or similar without breaking existing callers.

### Known fragilities (user-reported + audit)
1. **`OrderResponse.UnmarshalJSON`** only handles `orderID`/`id` string/number fallback. V2 may return additional fields or different numeric encodings.
2. **`PricesHistoryResponse.UnmarshalJSON`** handles `history` and `data` wrappers. V2 may introduce a third wrapper shape.
3. **`Trade`** struct still carries `FeeRateBps`. V2 trades may omit this or replace it with dynamic fee breakdowns.
4. **Hard-coded contract addresses** in `impl_orders.go` (`signOrderWithCreds`). No chain-configurable address resolution exists.
5. **`FeeRateBps`** is fetched and embedded into every order by `OrderBuilder`. V2 rejects orders with non-zero `feeRateBps` in the signed struct.
6. **Builder flow** (`auth.BuilderConfig`, HMAC headers) is pervasive in `pkg/auth/auth.go` and `pkg/transport/transport.go`. V2 removes all builder HMAC headers.
7. **No `ClobMarketInfo` endpoint** exists. V2 introduces this as the canonical way to read per-market parameters (tick size, fees, etc.).
8. **`MarketsResponse`** only unmarshals `"data"` as the array key. The new `GET /markets/keyset` endpoint uses `"markets"` (Apr 10, 2026).
9. **`MarketsRequest`** lacks a `Closed` field, but the API now defaults `closed=false` (Apr 9, 2026). Users who relied on the old default will see different results.
10. **`Market`** struct has no `FeeCurve` field. V2 market objects may embed fee information; `ClobMarketInfo` is the canonical source (Mar 31, 2026).

---

## 3. Detailed Implementation Plan

### Phase A: Core Types & Signatures (High Risk — Breaks Everything if Wrong)

#### 3.1 `pkg/clob/clobtypes/types.go` — Order struct

**Changes:**
```go
type Order struct {
    Salt          types.U256    `json:"salt"`
    Signer        types.Address `json:"signer"`
    Maker         types.Address `json:"maker"`
    Taker         types.Address `json:"taker"`          // KEEP in JSON for wire compat, but zero
    TokenID       types.U256    `json:"token_id"`
    MakerAmount   types.Decimal `json:"maker_amount"`
    TakerAmount   types.Decimal `json:"taker_amount"`
    Expiration    types.U256    `json:"expiration"`     // KEEP in JSON for wire compat, but zero
    Side          string        `json:"side"`
    FeeRateBps    types.Decimal `json:"fee_rate_bps"`   // KEEP in JSON for wire compat, but zero
    Nonce         types.U256    `json:"nonce"`          // KEEP in JSON for wire compat, but zero
    SignatureType *int          `json:"signature_type,omitempty"`

    // V2 additions
    Timestamp types.U256 `json:"timestamp"` // ms since epoch, replaces nonce for uniqueness
    Metadata  string     `json:"metadata"`  // bytes32 hex string
    Builder   string     `json:"builder"`   // bytes32 builder code, zero if unused
}
```

**Rationale:** The signed EIP-712 struct drops `taker`, `expiration`, `nonce`, `feeRateBps`, but the POST `/order` body example in the migration guide still includes them as zero values alongside the new fields. To avoid wire-format risk, we continue serializing the old fields as zero.

#### 3.2 `pkg/clob/impl_orders.go` — EIP-712 signing (`signOrderWithCreds`)

**Changes:**
1. **Domain version** `"1"` → `"2"`
2. **VerifyingContract** → V2 address (`0xE111180000d2663C0091e4f400237545B87B996B`)
3. **`typesDef["Order"]`** — remove `taker`, `expiration`, `nonce`, `feeRateBps`; add `timestamp`, `metadata`, `builder`
4. **`message`** construction — remove old fields, add:
   - `timestamp`: `(*math.HexOrDecimal256)(order.Timestamp.Int)` (must be ms)
   - `metadata`: `order.Metadata` (bytes32 string)
   - `builder`: `order.Builder` (bytes32 string)
5. **Address resolution** — hard-coded addresses must become configurable. Add a `chainID → contract map` or accept an override in `Client`.

**NegRisk markets:** The verifying contract is different for neg-risk. The current code has no concept of neg-risk order signing. We need:
```go
func (c *clientImpl) signOrder(order *clobtypes.Order, isNegRisk bool) (*clobtypes.SignedOrder, error)
```
or a `NegRisk` flag on the order that selects the correct contract address.

#### 3.3 `pkg/clob/order_payload.go` — Wire format

The `orderWithSignature` helper builds the JSON map for POST `/order`.

**Changes:**
- Continue emitting `taker`, `expiration`, `nonce`, `feeRateBps` as zero values for API compatibility.
- Add `timestamp`, `metadata`, `builder` to the map.
- Ensure `timestamp` is emitted as a string (the API example shows `"timestamp": "1713398400000"`).

#### 3.4 `pkg/clob/order_builder.go` & `order_builder_resolve.go`

**Changes:**
1. **Remove `FeeRateBps` builder methods** (`FeeRateBps`, `FeeRateBpsDec`). In V2, fees are protocol-determined at match time. Makers are never charged.
2. **Remove `Nonce` builder method**. Replace with internal `Timestamp` generation (ms since epoch).
3. **Remove `Taker` defaulting**. V2 signed struct has no `taker`, but wire body still sends zero address.
4. **`resolveFeeRateBps`** — currently called in `buildLimit` and `BuildMarket`. For V2:
   - Option A: Delete the call entirely (fees are not user-settable).
   - Option B: Keep it only for informational logging, but do **not** embed in the order.
   - Preferred: Delete. If users need fee info, they call the new `ClobMarketInfo` endpoint or read `Market.FeeCurve`.
5. **`BuildMarket`** — market buy orders in V2 accept an optional `userUSDCBalance` (or `userPUSDBalance`) so the SDK can calculate fee-adjusted fill amounts. Add:
   ```go
   func (b *OrderBuilder) UserBalance(balance float64) *OrderBuilder // or UserBalanceDec
   ```
   This is informational only; the actual fill amount is computed by the protocol.
6. **Add `BuilderCode` support**:
   ```go
   func (b *OrderBuilder) BuilderCode(code string) *OrderBuilder
   ```
   The code is a bytes32 string (or hex). It propagates to `order.Builder`.

#### 3.5 `pkg/auth/auth.go` — Builder HMAC removal

**Changes:**
1. **Remove `BuilderCredentials`, `BuilderRemoteConfig`, `BuilderConfig`** structs and all related code.
2. **Remove builder header constants:**
   - `HeaderPolyBuilderAPIKey`
   - `HeaderPolyBuilderPassphrase`
   - `HeaderPolyBuilderSignature`
   - `HeaderPolyBuilderTimestamp`
3. **Remove `BuildL2Headers` builder branch** (the transport no longer injects builder HMAC headers).

**Note:** `BuilderConfig` is referenced in:
- `pkg/clob/client.go` interface (`WithBuilderConfig`, `PromoteToBuilder`)
- `pkg/clob/impl.go` (`builderCfg` field, `WithBuilderConfig`, `PromoteToBuilder`)
- `pkg/transport/transport.go` (`builder` field, `SetBuilderConfig`, header injection)
- Root `client.go` (`builderCfg`, `WithAuth` chain)

These must all be refactored. The builder code moves **into the order** as a field, not as request headers.

#### 3.6 `pkg/transport/transport.go`

**Changes:**
- Remove `builder` field and `SetBuilderConfig`.
- Remove builder header injection in `doCall`.

---

### Phase B: API Surface & Endpoints

#### 3.7 `pkg/clob/constants.go` & `config.go` — Base URLs

**Key finding from changelog:** After April 28, `clob.polymarket.com` will **automatically serve V2**. No permanent default URL change is required.

**Changes:**
- Keep `BaseURL = "https://clob.polymarket.com"` as the default.
- Add `BaseURLV2 = "https://clob-v2.polymarket.com"` for pre-cutover testing.
- Update `cmd/acceptance` with a `--v2` flag that swaps the base URL.
- In integration tests, use `BaseURLV2`.

#### 3.8 `pkg/clob/client.go` & `impl_market.go` — New `ClobMarketInfo` endpoint

**Changes:**
Add to `Client` interface:
```go
ClobMarketInfo(ctx context.Context, tokenID string) (clobtypes.ClobMarketInfoResponse, error)
```

**Exact schema from API docs:**

```go
type ClobMarketInfoRequest struct {
    ConditionID string `json:"condition_id"`
}

type ClobMarketInfoResponse struct {
    GST        *string                `json:"gst,omitempty"`        // Game start time (ISO 8601), sports markets only
    Rewards    map[string]interface{} `json:"r,omitempty"`          // Rewards configuration (opaque — shape varies by market)
    Tokens     []ClobMarketToken      `json:"t,omitempty"`          // Tokens for this market
    MinOrderSize float64              `json:"mos,omitempty"`        // Minimum order size
    MinTickSize  float64              `json:"mts,omitempty"`        // Minimum tick size (price increment)
    MakerBaseFee int64                `json:"mbf,omitempty"`        // Maker base fee in basis points
    TakerBaseFee int64                `json:"tbf,omitempty"`        // Taker base fee in basis points
    RFQEnabled   bool                 `json:"rfqe,omitempty"`       // Whether RFQ is enabled
    TakerOrderDelayEnabled bool       `json:"itode,omitempty"`      // Whether taker order delay is enabled
    BlockaidCheckEnabled bool         `json:"ibce,omitempty"`       // Whether Blockaid check is enabled
    FeeCurve     *FeeCurve            `json:"fd,omitempty"`         // Fee curve parameters
    MinOrderAgeSeconds int64          `json:"oas,omitempty"`        // Minimum order age in seconds
}

type ClobMarketToken struct {
    TokenID string `json:"t"`          // Token ID
    Outcome string `json:"o"`          // Outcome name (e.g. "Yes")
}

type FeeCurve struct {
    Rate     float64 `json:"r,omitempty"`  // Fee rate
    Exponent int64   `json:"e,omitempty"`  // Exponent
    TakerOnly bool   `json:"to,omitempty"` // Taker-only flag
}
```

**Important:** The response uses **single-letter JSON keys** (e.g. `"t"` for tokens, `"fd"` for fee curve). This is unusual and easy to miss. Implement **defensive unmarshaling** with fallbacks for full-word keys in case Polymarket changes this.

#### 3.9 `pkg/clob/impl_market.go` — Fee endpoint changes

`FeeRate` still exists in V2 but returns dynamic market-specific fees. The old behavior (caching `feeRates` as `int64`) may still work, but the meaning changed: it is no longer embedded in orders. We should:
- Keep `FeeRate` / `FeeRateByPath` for user visibility.
- Remove `SetFeeRateBps` cache mutation from `clientImpl` (or keep as read-only cache).
- Update `OrderBuilder` to stop consulting `FeeRate` when constructing orders.

#### 3.10 `pkg/clob/impl_market.go` — Markets pagination & keyset endpoint

**Changelog finding (Apr 10, 2026):** New `GET /markets/keyset` replaces offset-based pagination. Wrapper response uses `"markets"` instead of `"data"`. Same filters, same item shape.

**Changes:**
1. Add `MarketsKeyset` to `Client` interface:
   ```go
   MarketsKeyset(ctx context.Context, req *clobtypes.MarketsKeysetRequest) (clobtypes.MarketsKeysetResponse, error)
   ```
2. Add request/response types:
   ```go
   type MarketsKeysetRequest struct {
       Limit       int    `json:"limit,omitempty"`
       AfterCursor string `json:"after_cursor,omitempty"`
       Active      *bool  `json:"active,omitempty"`
       AssetID     string `json:"asset_id,omitempty"`
       Closed      *bool  `json:"closed,omitempty"`
   }
   type MarketsKeysetResponse struct {
       Markets    []Market `json:"markets"`
       NextCursor string   `json:"next_cursor"`
       Limit      int      `json:"limit"`
       Count      int      `json:"count"`
   }
   ```
3. Harden `MarketsResponse.UnmarshalJSON` to accept both `"data"` and `"markets"` as the array key. This lets `Markets()` tolerate either endpoint shape.
4. Add `Closed *bool` to `MarketsRequest` to match the Apr 9 API change.

#### 3.11 `pkg/clob/clobtypes/types.go` — Market struct additions

**Changelog finding (Mar 31, 2026):** Fees are now provided via `feeSchedule` inside market objects.

**Changes:**
```go
type Market struct {
    ID             string        `json:"id"`
    Question       string        `json:"question"`
    ConditionID    string        `json:"condition_id"`
    // ... existing fields ...
    FeeCurve       *FeeCurve     `json:"fee_curve,omitempty"`     // From ClobMarketInfo; not always present in Market response
}

type FeeCurve struct {
    Rate      float64 `json:"r,omitempty"`
    Exponent  int64   `json:"e,omitempty"`
    TakerOnly bool    `json:"to,omitempty"`
}
```

**Note:** The `FeeCurve` is returned by `GET /clob-markets/{condition_id}` under key `fd`. It is not currently embedded in the `Market` struct from `GET /markets`, but we include it with `omitempty` for forward compatibility. Use defensive unmarshaling so unknown fee shapes don't break the whole `Market` parse.

#### 3.12 `pkg/clob/impl_account.go` — Collateral token

Balance/allowance queries currently default to `"USDC"`. V2 uses pUSD.

**Changes:**
- Update examples and documentation to reference `"pUSD"` or `"USDC"` depending on the target chain/date.
- The `AssetTypeCollateral` constant and `BalanceAllowanceRequest` struct do not need changes, but default strings in examples should be updated.

---

### Phase C: WebSocket & Streaming

#### 3.13 `pkg/clob/ws/types.go` & `events.go`

**WS endpoint confirmation (from AsyncAPI spec + tradecore repo):**
- Market channel: `wss://ws-subscriptions-clob.polymarket.com/ws/market`
- User channel: `wss://ws-subscriptions-clob.polymarket.com/ws/user`
- Our `config.go` already has the correct base host (`wss://ws-subscriptions-clob.polymarket.com`). Paths are `/ws/market` and `/ws/user`.

**Missing fields in current event structs (from AsyncAPI):**

```go
// TickSizeChangeEvent — add old/new tick size fields
type TickSizeChangeEvent struct {
    AssetID         string `json:"asset_id"`
    Market          string `json:"market,omitempty"`
    TickSize        string `json:"tick_size,omitempty"`         // legacy
    MinimumTickSize string `json:"minimum_tick_size,omitempty"` // legacy
    OldTickSize     string `json:"old_tick_size,omitempty"`     // NEW from AsyncAPI
    NewTickSize     string `json:"new_tick_size,omitempty"`     // NEW from AsyncAPI
    Timestamp       string `json:"timestamp,omitempty"`
}

// NewMarketEvent — many new optional fields
type NewMarketEvent struct {
    ID                   string        `json:"id"`
    Question             string        `json:"question"`
    Market               string        `json:"market,omitempty"`
    Slug                 string        `json:"slug,omitempty"`
    Description          string        `json:"description,omitempty"`
    AssetIDs             []string      `json:"assets_ids,omitempty"`
    Outcomes             []string      `json:"outcomes,omitempty"`
    EventMessage         *EventMessage `json:"event_message,omitempty"`
    Timestamp            string        `json:"timestamp,omitempty"`
    Tags                 []string      `json:"tags,omitempty"`                    // NEW
    ConditionID          string        `json:"condition_id,omitempty"`            // NEW
    Active               *bool         `json:"active,omitempty"`                  // NEW
    ClobTokenIDs         []string      `json:"clob_token_ids,omitempty"`          // NEW
    SportsMarketType     string        `json:"sports_market_type,omitempty"`      // NEW
    Line                 string        `json:"line,omitempty"`                    // NEW
    GameStartTime        string        `json:"game_start_time,omitempty"`         // NEW
    OrderPriceMinTickSize string       `json:"order_price_min_tick_size,omitempty"` // NEW
    GroupItemTitle       string        `json:"group_item_title,omitempty"`        // NEW
}

// MarketResolvedEvent — add tags
type MarketResolvedEvent struct {
    // ... existing fields ...
    Tags []string `json:"tags,omitempty"` // NEW
}
```

**User Channel `TradeEvent` (from API docs):**
The user channel trade event includes a `maker_orders` array with `side` field (June 2025 changelog). Our flat `TradeEvent` struct in `ws/types.go` should use `omitempty` and the dispatcher should tolerate nested `maker_orders` without crashing.

**Other changes:**
- `OrderEvent` may receive new V2 fields (`timestamp` as ms, `metadata`, `builder`). Extend with `json:",omitempty"`.
- `TradeEvent` may lose `FeeRateBps` or gain dynamic fee fields. Add `omitempty`.
- `LastTradePriceEvent` may include `transaction_hash`. Add `omitempty`.
- The `processEvent` dispatcher already re-marshals JSON, which safely tolerates unknown fields.

---

### Phase D: Defensive Response Handling

The user noted that "even some of the current implementation is erroring on unexpected responses." We should systematically add defensive unmarshaling to every response type that interacts with V2.

#### 3.14 Response hardening checklist

| Struct | Current Defense | V2 Action |
|--------|-----------------|-----------|
| `OrderResponse` | Custom `UnmarshalJSON` (string/number for time fields) | Extend to handle missing `expiration`, new `timestamp` ms string, new `metadata`/`builder` fields |
| `PricesHistoryResponse` | Custom `UnmarshalJSON` (array vs object wrapper) | Extend to handle new wrapper keys |
| `healthResponse` | Custom `UnmarshalJSON` (string vs object) | Extend if V2 returns a different shape |
| `MarketsResponse` | Standard JSON | **Add `UnmarshalJSON`** to tolerate `"data"` vs `"markets"` wrapper keys |
| `Market` | Standard JSON | **Add `UnmarshalJSON`** or use `json.RawMessage` for `FeeCurve` to tolerate unknown shapes |
| `OrderBook` | Standard JSON | Add `UnmarshalJSON` to tolerate missing `min_order_size` or new fields |
| `Trade` | Standard JSON | Add `UnmarshalJSON` to tolerate missing `fee_rate_bps` or new `MakerOrder` sub-objects |
| `TickSizeResponse` | Standard JSON (falls back `MinimumTickSize` → `TickSize`) | Keep fallback; add new fields if any |
| `FeeRateResponse` | Standard JSON | Keep; V2 may add fields |
| `ClobMarketInfoResponse` *(new)* | — | **Must** implement custom `UnmarshalJSON` for single-letter keys (`t`, `fd`, `mos`, etc.) with full-word fallbacks |

**Pattern to apply:**
```go
func (r *SomeResponse) UnmarshalJSON(data []byte) error {
    type Alias SomeResponse // avoid recursion
    var aux Alias
    if err := json.Unmarshal(data, &aux); err == nil {
        *r = SomeResponse(aux)
        return nil
    }
    // fallback: try alternate shapes, populate what we can
    // never return error if at least the critical fields are parseable
}
```

---

### Phase E: Examples & Documentation

#### 3.15 Examples to update

| Example | Changes |
|---------|---------|
| `examples/trading/main.go` | Remove `FeeRateBps`, `Nonce`, `Taker`, `Expiration` from manual `Order` struct; add `Timestamp` generation |
| `examples/order_builder/main.go` | Remove `FeeRateBps(0)` call; update printed fields |
| `examples/builder_flow/main.go` | **Major rewrite.** Remove HMAC builder auth. Show `BuilderCode("0x...")` on `OrderBuilder` |
| `examples/market_order/main.go` | Show `UserBalance` option for fee-aware market orders |
| `examples/gtd_order/main.go` | GTD still supported; `Expiration` moves from signed struct to order options only? Verify with API. |

---

## 4. Testing Plan (Live API Validation)

> **Goal:** Prove the SDK works against `clob-v2.polymarket.com` before cutover day without risking real funds.

### 4.1 Environment Setup

Create a new test suite: `pkg/clob/v2_integration_test.go` (build-tagged `//go:build integration`):

```go
//go:build integration

package clob

const testBaseURL = "https://clob-v2.polymarket.com"
```

Run with: `go test ./pkg/clob -tags integration -run TestV2 -v`

### 4.2 Public Endpoint Checks (No Auth Required)

Use the **test markets** from the migration guide:

| Market | Token ID (excerpt) |
|--------|-------------------|
| US / Iran nuclear deal in 2027? | `102936…7216` |
| Highest grossing movie in 2026? | `81662…2777`, `17546…1707`, etc. |

**Test cases:**

```go
func TestV2PublicEndpoints(t *testing.T) {
    client := NewClient(transport.NewClient(nil, testBaseURL))
    ctx := context.Background()
    tokenID := "102936224134271070189104847090829839924697394514566827387181305960175107677216"

    t.Run("Time", func(t *testing.T) { /* call Time, assert ts > 0 */ })
    t.Run("Health", func(t *testing.T) { /* call Health */ })
    t.Run("Markets", func(t *testing.T) { /* Markets(limit=5) */ })
    t.Run("MarketsKeyset", func(t *testing.T) { /* MarketsKeyset(limit=5) */ })
    t.Run("OrderBook", func(t *testing.T) { /* OrderBook(tokenID) */ })
    t.Run("Midpoint", func(t *testing.T) { /* Midpoint(tokenID) */ })
    t.Run("Price", func(t *testing.T) { /* Price BUY/SELL */ })
    t.Run("Spread", func(t *testing.T) { /* Spread */ })
    t.Run("LastTradePrice", func(t *testing.T) { /* LastTradePrice */ })
    t.Run("TickSize", func(t *testing.T) { /* TickSize */ })
    t.Run("NegRisk", func(t *testing.T) { /* NegRisk */ })
    t.Run("FeeRate", func(t *testing.T) { /* FeeRate */ })
    t.Run("ClobMarketInfo", func(t *testing.T) { /* NEW endpoint */ })
    t.Run("PricesHistory", func(t *testing.T) { /* PricesHistory */ })
}
```

**Success criteria:** No `json.Unmarshal` errors. If the API returns a field we don't recognize, the test should still pass (validates our defensive unmarshaling).

### 4.3 Authenticated Read-Only Checks

Requires `POLYMARKET_PK` + `POLYMARKET_API_KEY` + `POLYMARKET_API_SECRET` + `POLYMARKET_API_PASSPHRASE`.

**Test cases:**
```go
func TestV2AuthenticatedReads(t *testing.T) {
    // Auth with V2 credentials (same L1/L2 flow, unchanged)
    client := NewClient(transport.NewClient(nil, testBaseURL)).WithAuth(signer, apiKey)

    t.Run("BalanceAllowance_pUSD", func(t *testing.T) {
        // Query pUSD balance, not USDC
    })
    t.Run("Orders", func(t *testing.T) { /* Orders(limit=1) */ })
    t.Run("Trades", func(t *testing.T) { /* Trades(limit=1) */ })
    t.Run("UserEarnings", func(t *testing.T) { /* UserEarnings */ })
    t.Run("ClosedOnlyStatus", func(t *testing.T) { /* ClosedOnlyStatus */ })
}
```

### 4.4 Order Signing Validation (No Funds at Risk)

**Test case:** Build and sign an order, but **do not post it**. Verify the EIP-712 signature validates against the V2 domain and struct hash locally.

```go
func TestV2OrderSignature(t *testing.T) {
    // 1. Build order with OrderBuilder
    // 2. Sign with signOrderWithCreds
    // 3. Recover signer from signature using V2 domain & Order type hash
    // 4. Assert recovered address == signer.Address()
}
```

This is the **most important test**. If the signature is wrong, every `PostOrder` will return `INVALID_SIGNATURE`.

### 4.5 Live Order Placement (Minimal Risk)

**Prerequisite:** A funded test wallet with pUSD. Polymarket does not mention a separate testnet; the test markets are on the production V2 endpoint pre-cutover.

**Test case (tiny size, immediately cancel):**
```go
func TestV2LiveOrderRoundTrip(t *testing.T) {
    t.Skip("Requires funded wallet with pUSD — run manually")

    // 1. Build a tiny GTC limit order (e.g., 1 share at $0.01 on a test market)
    // 2. PostOrder
    // 3. Assert Status is LIVE or PENDING
    // 4. CancelOrder
    // 5. Assert cancellation accepted
}
```

**Safety rules:**
- Use the **test markets** listed in the migration guide (low liquidity, low value).
- Order size = minimum allowed (verify via `ClobMarketInfo`).
- Price far from mid (unlikely to fill).
- Cancel immediately after placement.

### 4.6 Market Order Validation

```go
func TestV2MarketOrderBuild(t *testing.T) {
    // Build a market buy order with UserBalance set
    // Verify the signed order has:
    //   - timestamp > 0
    //   - builder = user-supplied code (or zero)
    //   - feeRateBps = 0 in message (not signed)
    //   - taker = zero addr in JSON but NOT in signed struct
}
```

### 4.7 Builder Code Flow

```go
func TestV2BuilderCode(t *testing.T) {
    // 1. OrderBuilder.BuilderCode("0x1234...")
    // 2. Build & sign
    // 3. Verify order.Builder == "0x1234..."
    // 4. Verify no builder HMAC headers are sent by transport
}
```

### 4.8 NegRisk Order Signing

```go
func TestV2NegRiskOrderSignature(t *testing.T) {
    // Build order for a neg-risk token
    // Verify the verifyingContract in the EIP-712 domain is the V2 neg-risk address
}
```

### 4.9 WebSocket Connection

```go
func TestV2WSConnection(t *testing.T) {
    // Connect to V2 WS endpoint
    // Subscribe to orderbook for a test market token
    // Wait for first snapshot or heartbeat
    // Assert no protocol errors
}
```

### 4.10 Regression Matrix

After all V2 changes, run the existing unit test suite (`go test ./...`) and ensure:
- No compile errors.
- Mock-based tests still pass (may need mock updates for new `Order` fields).
- The `cmd/acceptance` tool gets a `--v2` flag so we can run the same checks against both hosts.

---

## 5. Execution Order (Recommended)

| Step | Task | Risk | Est. Effort |
|------|------|------|-------------|
| 1 | Add `BaseURLV2` constant; keep default unchanged. Add `--v2` to acceptance tool. | Low | 1h |
| 2 | Refactor `Order` struct: add `Timestamp`, `Metadata`, `Builder`; keep old fields for JSON | Low | 2h |
| 3 | Update `signOrderWithCreds` with V2 EIP-712 struct & domain version `"2"` | **Critical** | 4h |
| 4 | Make verifying contracts configurable (regular + neg-risk) | Medium | 2h |
| 5 | Update `orderWithPayload` to emit V2 fields in POST body | Low | 1h |
| 6 | Remove `FeeRateBps` from `OrderBuilder`; remove `Nonce` builder method; auto-gen `Timestamp` | Medium | 3h |
| 7 | Add `BuilderCode` to `OrderBuilder`; remove `BuilderConfig` / HMAC from `auth` and `transport` | Medium | 4h |
| 8 | Add `ClobMarketInfo` endpoint with defensive unmarshaling | Low | 2h |
| 9 | Add `MarketsKeyset` endpoint; harden `MarketsResponse` for `"markets"` wrapper | Low | 2h |
| 10 | Add `FeeCurve` to `Market`; add `Closed` to `MarketsRequest` | Low | 1h |
| 11 | Harden all response types (`UnmarshalJSON` fallbacks) | Low | 3h |
| 12 | Update WS event types with `omitempty` for new/missing fields | Low | 1h |
| 13 | Write `v2_integration_test.go` (public + auth reads + signature recovery) | Low | 4h |
| 14 | Run integration tests against `clob-v2.polymarket.com`; iterate on response shapes | Medium | 4h |
| 15 | Execute `TestV2LiveOrderRoundTrip` with minimal pUSD | **High** (funds) | 2h |
| 16 | Update examples & `cmd/acceptance` | Low | 3h |
| 17 | Tag release (default URL stays `clob.polymarket.com` — it becomes V2 on cutover) | Low | 1h |

**Total estimate:** ~40 engineering hours + coordination time for live testing.

---

## 6. Backward Compatibility & Rollout

### Option A: Hard Cutover (Recommended for this SDK)
Polymarket states there is **no backward compatibility**. V1 stops working at cutover. Therefore:
- Release a **new major version** of the Go SDK (e.g., `v2.0.0`).
- Document that `v1.x` is frozen and only works pre-cutover.

### Base URL Strategy
Because `clob.polymarket.com` auto-switches to V2 on April 28, the SDK **does not need a permanent default URL change**. Users running the new SDK before cutover should override the base URL to `clob-v2.polymarket.com` for testing. After cutover, the default works unchanged.

### Deprecations
- `OrderBuilder.FeeRateBps*` → remove (no replacement; fees are protocol-set).
- `OrderBuilder.Nonce` → remove (replaced by auto `Timestamp`).
- `Client.WithBuilderConfig` / `PromoteToBuilder` → remove (replaced by `OrderBuilder.BuilderCode`).
- `auth.BuilderConfig` → remove.

---

## 7. Open Questions to Resolve Before Coding

1. ✅ **V2 WebSocket endpoint:** Confirmed from AsyncAPI spec and tradecore repo. Market channel: `wss://ws-subscriptions-clob.polymarket.com/ws/market`. User channel: `wss://ws-subscriptions-clob.polymarket.com/ws/user`. Same host as V1.
2. ✅ **`ClobMarketInfo` exact response shape:** Resolved from API docs. Uses single-letter keys (`t`, `fd`, `mos`, etc.). See Section 3.8 for full schema.
3. **GTD orders in V2:** Does `expiration` move to order metadata (outside the signed struct) or is it still part of the wire body only?
4. **pUSD wrapping:** Should the SDK provide a helper to call `CollateralOnramp.wrap()`? This is onchain, not CLOB API. Currently out of scope unless requested.
5. **Safe/Proxy wallet addresses:** Did the V2 factories change? The migration guide only mentions Exchange contracts.
6. ✅ **`FeeCurve` exact shape:** Resolved from `ClobMarketInfo` docs. Fields: `r` (rate), `e` (exponent), `to` (taker-only). See `FeeCurve` struct in Section 3.8.

---

## 8. Appendix: V2 Contract Reference

| Network | Type | Address |
|---------|------|---------|
| Polygon Mainnet | Exchange | `0xE111180000d2663C0091e4f400237545B87B996B` |
| Polygon Mainnet | NegRisk Exchange | `0xe2222d279d744050d28e00520010520000310F59` |
| Polygon Mainnet | CollateralOnramp (pUSD) | *TBD — see Polymarket docs* |

---

## 9. Changelog-Driven Addenda

Based on [docs.polymarket.com/changelog](https://docs.polymarket.com/changelog) (reviewed Apr 22, 2026):

| Date | Change | Impact on Plan |
|------|--------|----------------|
| Apr 21, 2026 | Relayer `POST /submit` returns immediately without `transactionHash` | No direct impact (SDK does not use relayer API), but note for future bridge/relayer integrations. |
| Apr 17, 2026 | V2 go-live date confirmed; `clob.polymarket.com` auto-switches | Confirms base URL strategy: keep default, add V2 override for testing. |
| Apr 10, 2026 | `GET /markets/keyset` and `GET /events/keyset` added | Add `MarketsKeyset` method; harden `MarketsResponse` for `"markets"` wrapper. |
| Apr 9, 2026 | `GET /markets` defaults to `closed=false` | Add `Closed *bool` to `MarketsRequest`. |
| Mar 31, 2026 | Fees calculated from `feeSchedule` object within market | Add `FeeCurve` to `Market` struct; `ClobMarketInfo` is canonical source. |
| Mar 30, 2026 | Fee Structure V2: per-category rates | Reinforces that fees are dynamic and market-specific. |
| July 23, 2025 | `min_order_size`, `neg_risk`, `tick_size` added to book response | Already present in `OrderBook` struct. Confirmed compatible. |
| June 3, 2025 | `side` field added to `MakerOrder` in trade objects | `Trade` struct may receive extra keys; defensive unmarshaling handles this. |
| May 28, 2025 | WS `initial_dump` field added | Already supported in `SubscriptionRequest`. |

---

## 10. Summary Checklist

- [x] Order struct updated with `Timestamp`, `Metadata`, `Builder`
- [x] EIP-712 domain version `"2"` and V2 contract addresses
- [x] Signed `Order` type hash excludes `taker/expiration/nonce/feeRateBps`
- [x] POST body includes old fields zeroed + new V2 fields
- [x] `OrderBuilder` no longer sets `feeRateBps` or `nonce`; auto-sets `timestamp`
- [x] `OrderBuilder` supports `BuilderCode`
- [x] `BuilderConfig` / HMAC headers fully removed from `auth` and `transport`
- [x] `ClobMarketInfo` endpoint added with defensive unmarshaling
- [x] `MarketsKeyset` endpoint added; `MarketsResponse` hardened for `"markets"` wrapper
- [x] `Market.FeeCurve` added; `MarketsRequest.Closed` added
- [x] Default base URL kept as `clob.polymarket.com` (auto-switches on cutover)
- [x] All response types hardened against unexpected fields
- [ ] Integration tests pass against `clob-v2.polymarket.com`
- [ ] Live order round-trip validated with minimal pUSD
- [x] Examples updated
- [x] `cmd/acceptance` supports `--v2` flag
- [ ] Major version bump (`v2.0.0`) tagged
