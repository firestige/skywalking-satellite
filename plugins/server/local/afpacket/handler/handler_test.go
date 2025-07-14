// Licensed to Apache Software Foundation (ASF) under one or more contributor
// license agreements. See the NOTICE file distributed with
// this work for additional information regarding copyright
// ownership. Apache Software Foundation (ASF) licenses this file to you under
// the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package handler

import (
	"testing"

	"github.com/apache/skywalking-satellite/plugins/server/local/afpacket/types"
	"github.com/stretchr/testify/assert"
)

func TestAllHandlers_Implementation(t *testing.T) {
	handlers := []types.PacketHandler{
		NewHTTPHandler(),
		NewSIPHandler(),
		NewESLHandler(),
	}

	for _, handler := range handlers {
		t.Run(handler.Name(), func(t *testing.T) {
			// Test that all handlers implement the interface correctly
			assert.NotEmpty(t, handler.Name())
			assert.NotEmpty(t, handler.Type())
			assert.Equal(t, handler.Name(), handler.Type())

			// Test stats
			stats := handler.Stats()
			assert.Equal(t, uint64(0), stats.PacketsHandled)
			assert.Equal(t, uint64(0), stats.DataGenerated)
			assert.Equal(t, uint64(0), stats.ErrorCount)
		})
	}
}
