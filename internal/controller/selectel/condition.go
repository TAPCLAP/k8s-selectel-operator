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

package selectel

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func setCondition(conditions *[]metav1.Condition, generation int64, condType string, status metav1.ConditionStatus, reason, message string) bool {
	now := metav1.Now()
	for i := range *conditions {
		existing := &(*conditions)[i]
		if existing.Type != condType {
			continue
		}
		if existing.Status == status && existing.Reason == reason && existing.Message == message && existing.ObservedGeneration == generation {
			return false
		}
		if existing.Status != status {
			existing.LastTransitionTime = now
		}
		existing.Status = status
		existing.Reason = reason
		existing.Message = message
		existing.ObservedGeneration = generation
		return true
	}
	*conditions = append(*conditions, metav1.Condition{
		Type:               condType,
		Status:             status,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: generation,
		LastTransitionTime: now,
	})
	return true
}
