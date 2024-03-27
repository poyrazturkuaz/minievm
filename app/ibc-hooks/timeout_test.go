package evm_hooks_test

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	transfertypes "github.com/cosmos/ibc-go/v8/modules/apps/transfer/types"
	channeltypes "github.com/cosmos/ibc-go/v8/modules/core/04-channel/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/holiman/uint256"
	"github.com/initia-labs/minievm/x/evm/contracts/counter"
	"github.com/stretchr/testify/require"
)

func Test_onTimeoutIcs20Packet_noMemo(t *testing.T) {
	ctx, input := createDefaultTestInput(t)
	_, _, addr := keyPubAddr()
	_, _, addr2 := keyPubAddr()

	data := transfertypes.FungibleTokenPacketData{
		Denom:    "foo",
		Amount:   "10000",
		Sender:   addr.String(),
		Receiver: addr2.String(),
		Memo:     "",
	}

	dataBz, err := json.Marshal(&data)
	require.NoError(t, err)

	err = input.IBCHooksMiddleware.OnTimeoutPacket(ctx, channeltypes.Packet{
		Data: dataBz,
	}, addr)
	require.NoError(t, err)
}

func Test_onTimeoutIcs20Packet_memo(t *testing.T) {
	ctx, input := createDefaultTestInput(t)
	_, _, addr := keyPubAddr()
	evmAddr := common.BytesToAddress(addr.Bytes())

	codeBz, err := hex.DecodeString(strings.TrimPrefix(counter.CounterBin, "0x"))
	require.NoError(t, err)

	_, contractAddr, err := input.EVMKeeper.EVMCreate(ctx, evmAddr, codeBz)
	require.NoError(t, err)

	abi, err := counter.CounterMetaData.GetAbi()
	require.NoError(t, err)

	data := transfertypes.FungibleTokenPacketData{
		Denom:    "foo",
		Amount:   "10000",
		Sender:   addr.String(),
		Receiver: contractAddr.Hex(),
		Memo: fmt.Sprintf(`{
			"evm": {
				"async_callback": {
					"id": 99,
					"contract_address": "%s"
				}
			}
		}`, contractAddr.Hex()),
	}

	dataBz, err := json.Marshal(&data)
	require.NoError(t, err)

	// hook should not be called to due to acl
	err = input.IBCHooksMiddleware.OnTimeoutPacket(ctx, channeltypes.Packet{
		Data: dataBz,
	}, addr)
	require.NoError(t, err)

	// check the contract state
	queryInputBz, err := abi.Pack("count")
	require.NoError(t, err)
	queryRes, logs, err := input.EVMKeeper.EVMCall(ctx, evmAddr, contractAddr, queryInputBz)
	require.NoError(t, err)
	require.Equal(t, uint256.NewInt(0).Bytes32(), [32]byte(queryRes))
	require.Empty(t, logs)

	// set acl
	require.NoError(t, input.IBCHooksKeeper.SetAllowed(ctx, contractAddr[:], true))

	// success
	err = input.IBCHooksMiddleware.OnTimeoutPacket(ctx, channeltypes.Packet{
		Data: dataBz,
	}, addr)
	require.NoError(t, err)

	// check the contract state; increased by 99
	queryRes, logs, err = input.EVMKeeper.EVMCall(ctx, evmAddr, contractAddr, queryInputBz)
	require.NoError(t, err)
	require.Equal(t, uint256.NewInt(99).Bytes32(), [32]byte(queryRes))
	require.Empty(t, logs)
}