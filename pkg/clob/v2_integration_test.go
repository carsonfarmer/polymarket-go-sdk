//go:build integration

package clob

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/GoPolymarket/polymarket-go-sdk/pkg/auth"
	"github.com/GoPolymarket/polymarket-go-sdk/pkg/clob/clobtypes"
	"github.com/GoPolymarket/polymarket-go-sdk/pkg/transport"
)

func getV2BaseURL() string {
	if u := os.Getenv("POLYMARKET_V2_URL"); u != "" {
		return u
	}
	return "https://clob-v2.polymarket.com"
}

func newV2Client(t *testing.T) Client {
	t.Helper()
	return NewClient(transport.NewClient(nil, getV2BaseURL()))
}

// Test market tokens from the V2 migration docs (US / Iran nuclear deal in 2027?).
const testTokenID = "102936224134271070189104847090829839924697394514566827387181305960175107677216"
const testConditionID = "0x182390641d3b1b47cc64274b9da290efd04221c586651ba190880713da6347d9"

func TestV2Public_Time(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := newV2Client(t).Time(ctx)
	if err != nil {
		t.Fatalf("Time: %v", err)
	}
	if resp.Timestamp <= 0 {
		t.Fatalf("expected positive timestamp, got %d", resp.Timestamp)
	}
	t.Logf("server time: %d", resp.Timestamp)
}

func TestV2Public_Health(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	status, err := newV2Client(t).Health(ctx)
	if err != nil {
		t.Fatalf("Health: %v", err)
	}
	if status != "OK" {
		t.Fatalf("expected OK, got %s", status)
	}
	t.Logf("health: %s", status)
}

func TestV2Public_Markets(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	active := true
	resp, err := newV2Client(t).Markets(ctx, &clobtypes.MarketsRequest{Limit: 5, Active: &active})
	if err != nil {
		t.Fatalf("Markets: %v", err)
	}
	if len(resp.Data) == 0 {
		t.Fatalf("expected markets, got none")
	}
	t.Logf("got %d markets", len(resp.Data))
}



func TestV2Public_Market(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := newV2Client(t).Market(ctx, testConditionID)
	if err != nil {
		t.Fatalf("Market(%s): %v", testConditionID, err)
	}
	if resp.ConditionID == "" {
		t.Fatalf("expected condition_id in response")
	}
	t.Logf("market condition_id: %s", resp.ConditionID)
}

func TestV2Public_ClobMarketInfo(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := newV2Client(t).ClobMarketInfo(ctx, &clobtypes.ClobMarketInfoRequest{ConditionID: testConditionID})
	if err != nil {
		t.Fatalf("ClobMarketInfo(%s): %v", testConditionID, err)
	}
	t.Logf("ClobMarketInfo: mos=%v mts=%v mbf=%d tbf=%d tokens=%d",
		resp.MinOrderSize, resp.MinTickSize, resp.MakerBaseFee, resp.TakerBaseFee, len(resp.Tokens))
}

func TestV2Public_OrderBook(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := newV2Client(t).OrderBook(ctx, &clobtypes.BookRequest{TokenID: testTokenID})
	if err != nil {
		t.Fatalf("OrderBook(%s): %v", testTokenID, err)
	}
	t.Logf("orderbook bids=%d asks=%d", len(resp.Bids), len(resp.Asks))
}

func TestV2Public_PriceAndMidpoint(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client := newV2Client(t)

	price, err := client.Price(ctx, &clobtypes.PriceRequest{TokenID: testTokenID, Side: "BUY"})
	if err != nil {
		t.Logf("Price(BUY): %v", err)
	} else {
		t.Logf("price BUY: %s", price.Price)
	}

	mid, err := client.Midpoint(ctx, &clobtypes.MidpointRequest{TokenID: testTokenID})
	if err != nil {
		t.Logf("Midpoint: %v", err)
	} else {
		t.Logf("midpoint: %s", mid.Midpoint)
	}
}

func TestV2Public_TickSizeNegRiskFeeRate(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client := newV2Client(t)

	ts, err := client.TickSize(ctx, &clobtypes.TickSizeRequest{TokenID: testTokenID})
	if err != nil {
		t.Logf("TickSize: %v", err)
	} else {
		t.Logf("tick size: %v", ts.MinimumTickSize)
	}

	nr, err := client.NegRisk(ctx, &clobtypes.NegRiskRequest{TokenID: testTokenID})
	if err != nil {
		t.Logf("NegRisk: %v", err)
	} else {
		t.Logf("neg_risk: %v", nr.NegRisk)
	}

	fr, err := client.FeeRate(ctx, &clobtypes.FeeRateRequest{TokenID: testTokenID})
	if err != nil {
		t.Logf("FeeRate: %v", err)
	} else {
		t.Logf("fee_rate: base=%d str=%s", fr.BaseFee, fr.FeeRate)
	}
}

func TestV2Public_PricesHistory(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := newV2Client(t).PricesHistory(ctx, &clobtypes.PricesHistoryRequest{
		Market:   testConditionID,
		Interval: clobtypes.PriceHistoryInterval1d,
	})
	if err != nil {
		t.Fatalf("PricesHistory: %v", err)
	}
	t.Logf("price history points: %d", len(resp))
}

func TestV2Public_LastTradePrice(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := newV2Client(t).LastTradePrice(ctx, &clobtypes.LastTradePriceRequest{TokenID: testTokenID})
	if err != nil {
		t.Logf("LastTradePrice: %v", err)
	} else {
		t.Logf("last trade price: %s", resp.Price)
	}
}

// --- Order Signing Validation (no funds at risk) ---

func TestV2OrderSignature(t *testing.T) {
	pkHex := os.Getenv("POLYMARKET_PK")
	if pkHex == "" {
		t.Skip("POLYMARKET_PK not set")
	}

	signer, err := auth.NewPrivateKeySigner(pkHex, auth.PolygonChainID)
	if err != nil {
		t.Fatalf("signer: %v", err)
	}

	apiKey := &auth.APIKey{
		Key:        os.Getenv("POLYMARKET_API_KEY"),
		Secret:     os.Getenv("POLYMARKET_API_SECRET"),
		Passphrase: os.Getenv("POLYMARKET_API_PASSPHRASE"),
	}

	client := newV2Client(t).WithAuth(signer, apiKey)

	signable, err := NewOrderBuilder(client, signer).
		TokenID(testTokenID).
		Side("BUY").
		Price(0.01).
		Size(1).
		TickSize(0.01).
		BuilderCode("0x0000000000000000000000000000000000000000000000000000000000000000").
		BuildSignable()
	if err != nil {
		t.Fatalf("BuildSignable: %v", err)
	}

	if signable.Order.Timestamp == 0 {
		t.Fatalf("expected auto-generated timestamp")
	}
	if signable.Order.Builder == "" {
		t.Fatalf("expected builder code")
	}
	if signable.Order.FeeRateBps.BigInt() != nil && signable.Order.FeeRateBps.Sign() != 0 {
		t.Fatalf("expected feeRateBps to be zero in V2")
	}
	if signable.Order.Nonce.Int != nil && signable.Order.Nonce.Int.Sign() != 0 {
		t.Fatalf("expected nonce to be zero in V2")
	}
	// Verify metadata defaults to zero bytes32 in the wire payload
	if signable.Order.Metadata != "" {
		t.Fatalf("expected empty metadata to default to zero bytes32 in payload, got %s", signable.Order.Metadata)
	}

	// Sign but do NOT post — funds not at risk
	signed, err := SignOrder(signer, apiKey, signable.Order)
	if err != nil {
		t.Fatalf("SignOrder: %v", err)
	}
	if signed.Signature == "" {
		t.Fatalf("expected non-empty signature")
	}

	t.Logf("signed order: timestamp=%d builder=%s metadata=%s sig_len=%d",
		signed.Order.Timestamp, signed.Order.Builder, signed.Order.Metadata, len(signed.Signature))
}

// --- Geoblock ---

func TestV2Public_Geoblock(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := newV2Client(t).Geoblock(ctx)
	if err != nil {
		t.Logf("Geoblock: %v", err)
	} else {
		t.Log("Geoblock: OK")
	}
}
