// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package examples

import (
	"context"
	_ "embed"
	"testing"

	"github.com/stretchr/testify/require"
)

//go:embed testdata/pokemon.wasm
var pokemonProgramBytes []byte

// go test -v -timeout 30s -run ^TestPokemonProgram$ github.com/AnomalyFi/hypersdk/x/programs/examples
func TestPokemonProgram(t *testing.T) {
	require := require.New(t)
	program := NewPokemon(log, pokemonProgramBytes, maxGas, costMap)
	err := program.Run(context.Background())
	require.NoError(err)
}
