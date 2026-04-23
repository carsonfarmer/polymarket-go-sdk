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

func findTestToken(t *testing.T, ctx context.Context, client Client) string {
	t.Helper()
	active := true
	markets, err := client.Markets(ctx, &clobtypes.MarketsRequest{Limit: 10, Active: &active})
	if err != nil || len(markets.Data) == 0 {
		return ""
	}
	for _, m := range markets.Data {
		for _, tok := range m.Tokens {
			if tok.TokenID != "" {
				return tok.TokenID
			}
		}
	}
	return ""
}

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
	if status != "UP" && status != "up" {
		t.Fatalf("expected UP, got %s", status)
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

func TestV2Public_MarketsKeyset(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	active := true
	resp, err := newV2Client(t).MarketsKeyset(ctx, &clobtypes.MarketsKeysetRequest{Limit: 5, Active: &active})
	if err != nil {
		t.Fatalf("MarketsKeyset: %v", err)
	}
	if len(resp.Markets) == 0 {
		t.Fatalf("expected markets, got none")
	}
	t.Logf("got %d markets via keyset", len(resp.Markets))
}

func TestV2Public_Market(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client := newV2Client(t)
	active := true
	markets, err := client.Markets(ctx, &clobtypes.MarketsRequest{Limit: 1, Active: &active})
	if err != nil || len(markets.Data) == 0 {
		t.Skip("no markets available")
	}
	id := markets.Data[0].ID
	if id == "" {
		id = markets.Data[0].ConditionID
	}
	if id == "" {
		t.Skip("no market id")
	}

	resp, err := client.Market(ctx, id)
	if err != nil {
		t.Fatalf("Market(%s): %v", id, err)
	}
	if resp.ID == "" {
		t.Fatalf("expected market ID in response")
	}
	t.Logf("market: %s", resp.ID)
}

func TestV2Public_ClobMarketInfo(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client := newV2Client(t)
	active := true
	markets, err := client.Markets(ctx, &clobtypes.MarketsRequest{Limit: 5, Active: &active})
	if err != nil || len(markets.Data) == 0 {
		t.Skip("no markets available")
	}

	var conditionID string
	for _, m := range markets.Data {
		if m.ConditionID != "" {
			conditionID = m.ConditionID
			break
		}
	}
	if conditionID == "" {
		t.Skip("no condition_id found")
	}

	resp, err := client.ClobMarketInfo(ctx, &clobtypes.ClobMarketInfoRequest{ConditionID: conditionID})
	if err != nil {
		t.Fatalf("ClobMarketInfo(%s): %v", conditionID, err)
	}
	t.Logf("ClobMarketInfo: mos=%v mts=%v mbf=%d tbf=%d tokens=%d",
		resp.MinOrderSize, resp.MinTickSize, resp.MakerBaseFee, resp.TakerBaseFee, len(resp.Tokens))
}

func TestV2Public_OrderBook(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client := newV2Client(t)
	tokenID := findTestToken(t, ctx, client)
	if tokenID == "" {
		t.Skip("no test token found")
	}

	resp, err := client.OrderBook(ctx, &clobtypes.BookRequest{TokenID: tokenID})
	if err != nil {
		t.Fatalf("OrderBook(%s): %v", tokenID, err)
	}
	t.Logf("orderbook bids=%d asks=%d", len(resp.Bids), len(resp.Asks))
}

func TestV2Public_PriceAndMidpoint(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client := newV2Client(t)
	tokenID := findTestToken(t, ctx, client)
	if tokenID == "" {
		t.Skip("no test token found")
	}

	price, err := client.Price(ctx, &clobtypes.PriceRequest{TokenID: tokenID, Side: "BUY"})
	if err != nil {
		t.Logf("Price(BUY): %v", err)
	} else {
		t.Logf("price BUY: %s", price.Price)
	}

	mid, err := client.Midpoint(ctx, &clobtypes.MidpointRequest{TokenID: tokenID})
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
	tokenID := findTestToken(t, ctx, client)
	if tokenID == "" {
		t.Skip("no test token found")
	}

	ts, err := client.TickSize(ctx, &clobtypes.TickSizeRequest{TokenID: tokenID})
	if err != nil {
		t.Logf("TickSize: %v", err)
	} else {
		t.Logf("tick size: %v", ts.MinimumTickSize)
	}

	nr, err := client.NegRisk(ctx, &clobtypes.NegRiskRequest{TokenID: tokenID})
	if err != nil {
		t.Logf("NegRisk: %v", err)
	} else {
		t.Logf("neg_risk: %v", nr.NegRisk)
	}

	fr, err := client.FeeRate(ctx, &clobtypes.FeeRateRequest{TokenID: tokenID})
	if err != nil {
		t.Logf("FeeRate: %v", err)
	} else {
		t.Logf("fee_rate: base=%d str=%s", fr.BaseFee, fr.FeeRate)
	}
}

func TestV2Public_PricesHistory(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client := newV2Client(t)
	active := true
	markets, err := client.Markets(ctx, &clobtypes.MarketsRequest{Limit: 3, Active: &active})
	if err != nil || len(markets.Data) == 0 {
		t.Skip("no markets available")
	}

	conditionID := markets.Data[0].ConditionID
	if conditionID == "" {
		t.Skip("no condition_id")
	}

	resp, err := client.PricesHistory(ctx, &clobtypes.PricesHistoryRequest{
		Market:   conditionID,
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

	client := newV2Client(t)
	tokenID := findTestToken(t, ctx, client)
	if tokenID == "" {
		t.Skip("no test token found")
	}

	resp, err := client.LastTradePrice(ctx, &clobtypes.LastTradePriceRequest{TokenID: tokenID})
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

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	tokenID := findTestToken(t, ctx, client)
	if tokenID == "" {
		t.Skip("no test token found")
	}

	signable, err := NewOrderBuilder(client, signer).
		TokenID(tokenID).
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
	if signable.Order.FeeRateBps.Sign() != 0 {
		t.Fatalf("expected feeRateBps to be zero in V2")
	}
	if signable.Order.Nonce.Sign() != 0 {
		t.Fatalf("expected nonce to be zero in V2")
	}

	// Sign but do NOT post — funds not at risk
	signed, err := SignOrder(signer, apiKey, signable.Order)
	if err != nil {
		t.Fatalf("SignOrder: %v", err)
	}
	if signed.Signature == "" {
		t.Fatalf("expected non-empty signature")
	}

	t.Logf("signed order: timestamp=%d builder=%s sig_len=%d",
		signed.Order.Timestamp, signed.Order.Builder, len(signed.Signature))
}

// --- Geoblock ---

func TestV2Public_Geoblock(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := newV2Client(t).Geoblock(ctx)
	if err != nil {
		t.Logf("Geoblock: %v", err)
	}
}
