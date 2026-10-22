/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package cdnselectel

import (
	"testing"
	"time"
)

func TestProcessingDelayStaysWithinJitter(t *testing.T) {
	t.Parallel()

	cases := []time.Duration{0, 3 * time.Minute, 15 * time.Second}
	for _, interval := range cases {
		reconciler := &ResourceReconciler{ProcessingInterval: interval}
		base := interval
		if base <= 0 {
			base = defaultProcessingInterval
		}
		for range 200 {
			delay := reconciler.processingDelay()
			if delay < base || delay > base+processingJitter {
				t.Fatalf("interval %s produced %s", interval, delay)
			}
		}
	}
}
