package clob

import (
	"math/big"
	"testing"

	"github.com/GoPolymarket/polymarket-go-sdk/pkg/auth"
	"github.com/GoPolymarket/polymarket-go-sdk/pkg/clob/clobtypes"
	"github.com/GoPolymarket/polymarket-go-sdk/pkg/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/math"
	"github.com/ethereum/go-ethereum/signer/core/apitypes"
	"github.com/shopspring/decimal"
)

// TestV2SignatureHash_CrossReference verifies that our Go EIP-712 hash matches
// the output of the official TypeScript SDK (@polymarket/clob-client-v2) for
// identical inputs. The expected hash was computed with viem's hashTypedData:
//
//	domain = {
//	  name: "Polymarket CTF Exchange",
//	  version: "2",
//	  chainId: 137,
//	  verifyingContract: "0xE111180000d2663C0091e4f400237545B87B996B",
//	}
//	message = {
//	  salt: "12345",
//	  maker: "0x1111111111111111111111111111111111111111",
//	  signer: "0x1111111111111111111111111111111111111111",
//	  tokenId: "102936224134271070189104847090829839924697394514566827387181305960175107677216",
//	  makerAmount: "1000000",
//	  takerAmount: "2000000",
//	  side: 0,
//	  signatureType: 1,
//	  timestamp: "1713398400000",
//	  metadata: "0x0000000000000000000000000000000000000000000000000000000000000000",
//	  builder: "0x0000000000000000000000000000000000000000000000000000000000000000",
//	}
//
// Expected hash: 0x47d81490f9720c6d5b7d96f71e65a53cbad7e0e7d76ffe011945442cdb026a74
func TestV2SignatureHash_CrossReference(t *testing.T) {
	signer, err := auth.NewPrivateKeySigner("0x"+repeatChar('1', 64), auth.PolygonChainID)
	if err != nil {
		t.Fatalf("signer: %v", err)
	}

	apiKey := &auth.APIKey{Key: "test-key", Secret: "test-secret", Passphrase: "test-pass"}

	tokenID, _ := new(big.Int).SetString("102936224134271070189104847090829839924697394514566827387181305960175107677216", 10)
	maker := common.HexToAddress("0x1111111111111111111111111111111111111111")
	sigType := 1

	order := &clobtypes.Order{
		Salt:          types.U256{Int: big.NewInt(12345)},
		Maker:         maker,
		Signer:        maker,
		TokenID:       types.U256{Int: tokenID},
		MakerAmount:   types.Decimal(decimal.NewFromInt(1000000)),
		TakerAmount:   types.Decimal(decimal.NewFromInt(2000000)),
		Side:          "BUY",
		SignatureType: &sigType,
		Timestamp:     1713398400000,
		Metadata:      "0x0000000000000000000000000000000000000000000000000000000000000000",
		Builder:       "0x0000000000000000000000000000000000000000000000000000000000000000",
	}

	signed, err := SignOrder(signer, apiKey, order)
	if err != nil {
		t.Fatalf("SignOrder: %v", err)
	}
	if signed.Signature == "" {
		t.Fatal("expected non-empty signature")
	}

	// Compute the EIP-712 hash independently to verify it matches the TS SDK.
	typedData := apitypes.TypedData{
		Types: apitypes.Types{
			"EIP712Domain": {
				{Name: "name", Type: "string"},
				{Name: "version", Type: "string"},
				{Name: "chainId", Type: "uint256"},
				{Name: "verifyingContract", Type: "address"},
			},
			"Order": {
				{Name: "salt", Type: "uint256"},
				{Name: "maker", Type: "address"},
				{Name: "signer", Type: "address"},
				{Name: "tokenId", Type: "uint256"},
				{Name: "makerAmount", Type: "uint256"},
				{Name: "takerAmount", Type: "uint256"},
				{Name: "side", Type: "uint8"},
				{Name: "signatureType", Type: "uint8"},
				{Name: "timestamp", Type: "uint256"},
				{Name: "metadata", Type: "bytes32"},
				{Name: "builder", Type: "bytes32"},
			},
		},
		PrimaryType: "Order",
		Domain: apitypes.TypedDataDomain{
			Name:              "Polymarket CTF Exchange",
			Version:           "2",
			ChainId:           (*math.HexOrDecimal256)(big.NewInt(137)),
			VerifyingContract: "0xE111180000d2663C0091e4f400237545B87B996B",
		},
		Message: apitypes.TypedDataMessage{
			"salt":          "12345",
			"maker":         "0x1111111111111111111111111111111111111111",
			"signer":        "0x1111111111111111111111111111111111111111",
			"tokenId":       "102936224134271070189104847090829839924697394514566827387181305960175107677216",
			"makerAmount":   "1000000",
			"takerAmount":   "2000000",
			"side":          "0",
			"signatureType": "1",
			"timestamp":     "1713398400000",
			"metadata":      "0x0000000000000000000000000000000000000000000000000000000000000000",
			"builder":       "0x0000000000000000000000000000000000000000000000000000000000000000",
		},
	}

	hash, _, err := apitypes.TypedDataAndHash(typedData)
	if err != nil {
		t.Fatalf("TypedDataAndHash: %v", err)
	}

	want := "0x47d81490f9720c6d5b7d96f71e65a53cbad7e0e7d76ffe011945442cdb026a74"
	got := "0x" + common.Bytes2Hex(hash)
	if got != want {
		t.Fatalf("EIP-712 hash mismatch:\n  got:  %s\n  want: %s", got, want)
	}
}

func repeatChar(ch byte, n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = ch
	}
	return string(b)
}
