// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package trace

import (
	"go.opentelemetry.io/otel/trace/noop"

	"github.com/ava-labs/avalanchego/trace"
)

var _ trace.Tracer = (*noOpTracer)(nil)

// noOpTracer is an implementation of trace.Tracer that does nothing.
type noOpTracer struct {
	// t oteltrace.Tracer
	noop.Tracer
}

func (noOpTracer) Close() error {
	return nil
}
