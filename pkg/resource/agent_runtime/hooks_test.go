// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"). You may
// not use this file except in compliance with the License. A copy of the
// License is located at
//
//     http://aws.amazon.com/apache2.0/
//
// or in the "license" file accompanying this file. This file is distributed
// on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either
// express or implied. See the License for the specific language governing
// permissions and limitations under the License.

package agent_runtime

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"

	svcapitypes "github.com/aws-controllers-k8s/bedrockagentcorecontrol-controller/apis/v1alpha1"
)

func runtimeWith(version *string, description string) *resource {
	r := &resource{ko: &svcapitypes.AgentRuntime{}}
	r.ko.Spec.PlatformVersion = version
	r.ko.Spec.Description = aws.String(description)
	return r
}

// An unset platformVersion carries no intent, so it must not drive an update
// against the observed value; a set one still does.
func TestNewResourceDeltaPlatformVersion(t *testing.T) {
	observed := runtimeWith(aws.String("V1"), "d")
	if newResourceDelta(runtimeWith(nil, "d"), observed).DifferentAt("Spec.PlatformVersion") {
		t.Errorf("an unset platformVersion must not differ from the observed one")
	}
	if !newResourceDelta(runtimeWith(aws.String("V2"), "d"), observed).DifferentAt("Spec.PlatformVersion") {
		t.Errorf("a set platformVersion must still differ from the observed one")
	}
	delta := newResourceDelta(runtimeWith(nil, "changed"), observed)
	if !delta.DifferentAt("Spec.Description") || delta.DifferentAt("Spec.PlatformVersion") {
		t.Errorf("other differences must survive the filter: %v", delta.Differences)
	}
}
