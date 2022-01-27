// Copyright © 2021 Ettore Di Giacinto <mudler@mocaccino.org>
//
// This program is free software; you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation; either version 2 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License along
// with this program; if not, see <http://www.gnu.org/licenses/>.

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
