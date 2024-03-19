package p2ptests

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/zennittians/intelchain/p2p"
	"github.com/zennittians/intelchain/test/helpers"
)

func TestMultiAddressParsing(t *testing.T) {
	t.Parallel()

	multiAddresses, err := p2p.StringsToAddrs(helpers.Bootnodes)
	assert.NoError(t, err)
	assert.Equal(t, len(helpers.Bootnodes), len(multiAddresses))

	for index, multiAddress := range multiAddresses {
		assert.Equal(t, multiAddress.String(), helpers.Bootnodes[index])
	}
}

func TestAddressListConversionToString(t *testing.T) {
	t.Parallel()

	multiAddresses, err := p2p.StringsToAddrs(helpers.Bootnodes)
	assert.NoError(t, err)
	assert.Equal(t, len(helpers.Bootnodes), len(multiAddresses))

	expected := strings.Join(helpers.Bootnodes[:], ",")
	var addressList p2p.AddrList = multiAddresses
	assert.Equal(t, expected, addressList.String())
}

func TestAddressListConversionFromString(t *testing.T) {
	t.Parallel()

	multiAddresses, err := p2p.StringsToAddrs(helpers.Bootnodes)
	assert.NoError(t, err)
	assert.Equal(t, len(helpers.Bootnodes), len(multiAddresses))

	addressString := strings.Join(helpers.Bootnodes[:], ",")
	var addressList p2p.AddrList = multiAddresses
	addressList.Set(addressString)
	assert.Equal(t, len(addressList), len(multiAddresses))
	assert.Equal(t, addressList[0], multiAddresses[0])
}
