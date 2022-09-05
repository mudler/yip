//   Copyright 2021 Ettore Di Giacinto <mudler@mocaccino.org>
//
//   Licensed under the Apache License, Version 2.0 (the "License");
//   you may not use this file except in compliance with the License.
//   You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
//   Unless required by applicable law or agreed to in writing, software
//   distributed under the License is distributed on an "AS IS" BASIS,
//   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//   See the License for the specific language governing permissions and
//   limitations under the License.

package utils_test

import (
	. "github.com/mudler/yip/pkg/utils"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Utils", func() {
	Context("templates", func() {
		It("correctly templates input", func() {
			str, err := TemplatedString("foo-{{.}}", "bar")
			Expect(err).ShouldNot(HaveOccurred())
			Expect(str).Should(ContainSubstring("foo-bar"))
			Expect(len(str)).ToNot(Equal(4))

			str, err = TemplatedString("foo-", nil)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(str).Should(ContainSubstring("foo-"))
			Expect(len(str)).To(Equal(4))
		})
	})
	Context("random", func() {
		It("Generates strings of the correct length", func() {
			str := RandomString(5)
			Expect(len(str)).To(Equal(5))
			Expect(RandomString(5)).ToNot(Equal(str))
		})
	})
})
