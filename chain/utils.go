// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"slices"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/celestiaorg/nmt"

	"github.com/AnomalyFi/hypersdk/consts"
	"github.com/AnomalyFi/hypersdk/utils"
)

func CreateActionID(txID ids.ID, i uint8) ids.ID {
	actionBytes := make([]byte, ids.IDLen+consts.Uint8Len)
	copy(actionBytes, txID[:])
	actionBytes[ids.IDLen] = i
	return utils.ToID(actionBytes)
}

// return all tx data & a mapping from indexes from namespace to actions position(paired in (i, j), i-th tx and j-th action)
func ExtractTxsDataToApproveFromTxs(txs []*Transaction, results []*Result) ([][]byte, [][]byte, map[string][][2]int, error) {
	data2prove := make([][]byte, 0, len(txs))
	nmtNamespaceToTxIndexes := make(map[string][][2]int)
	nmtNSs := make([][]byte, 0, 10)

	for i := 0; i < len(txs); i++ {
		tx := txs[i]
		txID := tx.ID()
		txResult := results[i]

		if !txResult.Success {
			continue
		}
		for j := 0; j < len(tx.Actions); j++ {
			// record
			nID := tx.Actions[j].NMTNamespace()
			if _, ok := nmtNamespaceToTxIndexes[hex.EncodeToString(nID)]; !ok {
				nmtNamespaceToTxIndexes[hex.EncodeToString(nID)] = make([][2]int, 0, 1)
			}
			a := nmtNamespaceToTxIndexes[hex.EncodeToString(nID)]
			a = append(a, [2]int{i, j})
			nmtNamespaceToTxIndexes[hex.EncodeToString(nID)] = a

			txData := make([]byte, 0, 1+len(txID[:])+len(txResult.Outputs[j]))
			txData = append(txData, nID...)
			txData = append(txData, txID[:]...)
			for k := 0; k < len(txResult.Outputs[j]); k++ {
				txData = append(txData, txResult.Outputs[j][k]...)
			}
			data2prove = append(data2prove, txData)
		}
	}

	// sort txs data based on namespace id or NMT tree building will fail
	slices.SortFunc(data2prove, func(a, b []byte) int {
		return bytes.Compare(a[0:8], b[0:8])
	})

	for ns := range nmtNamespaceToTxIndexes {
		nID, err := hex.DecodeString(ns)
		if err != nil {
			return nil, nil, nil, ErrConvertingNamespace
		}
		nmtNSs = append(nmtNSs, nID)
	}

	return data2prove, nmtNSs, nmtNamespaceToTxIndexes, nil
}

// return (tree, root, proofs, error)
func BuildNMTTree(data [][]byte, namespaces [][]byte) (*nmt.NamespacedMerkleTree, []byte, map[string]nmt.Proof, error) {
	nmtTree := nmt.New(sha256.New())
	for _, d := range data {
		if err := nmtTree.Push(d); err != nil {
			return nil, nil, nil, ErrPushingElementInNMTTree
		}
	}

	nmtRoot, err := nmtTree.Root()
	if err != nil {
		return nil, nil, nil, ErrComputingNMTRoot
	}

	nmtProofs := make(map[string]nmt.Proof)
	for _, nID := range namespaces {
		proof, err := nmtTree.ProveNamespace(nID)
		if err != nil {
			continue
		}

		nmtProofs[hex.EncodeToString(nID)] = proof
	}

	return nmtTree, nmtRoot, nmtProofs, nil
}

// Check if txs from mempool contains any conflicting transactions from the anchor txs
func IsRepeatTxAsAnchorTx(anchorTxBitSet set.Set[ids.ID], marker set.Bits, txs []*Transaction) set.Bits {
	for i, tx := range txs {
		if anchorTxBitSet.Contains(tx.ID()) {
			marker.Add(i)
		}
	}
	return marker
}
