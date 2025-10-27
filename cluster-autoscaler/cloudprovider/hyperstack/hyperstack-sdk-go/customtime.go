/*
Copyright 2019 The Kubernetes Authors.

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
// Package hyperstack contains generated API client types for Hyperstack.
package hyperstack

import (
	"fmt"
	"strings"
	"time"
)

// CustomTime is a JSON marshal/unmarshal helper for API timestamps.
type CustomTime struct {
	time.Time
}

const ctLayout = "2006-01-02T15:04:05" // Specify your time format here

// UnmarshalJSON implements json.Unmarshaler for CustomTime.
func (ct *CustomTime) UnmarshalJSON(b []byte) (err error) {
	s := strings.Trim(string(b), "\"")
	if s == "null" {
		ct.Time = time.Time{}
		return
	}
	ct.Time, err = time.Parse(ctLayout, s)
	return
}

// MarshalJSON implements json.Marshaler for CustomTime.
func (ct *CustomTime) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf("\"%s\"", ct.Time.Format(ctLayout))), nil
}
