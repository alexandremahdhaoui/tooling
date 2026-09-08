// Copyright 2024 Alexandre Mahdhaoui
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

// ImageSource, BasicAuth and ValueFrom are generated from spec.openapi.yaml:
// the stage's spec is the one place they are declared, and forge validates
// a stage against it before anything runs. A BasicAuth with neither
// credential set is an absent one.
func hasBasicAuth(img ImageSource) bool {
	return img.BasicAuth != BasicAuth{}
}
