// Copyright © 2020 Ettore Di Giacinto <mudler@gentoo.org>
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

package schema_test

import (
	. "github.com/mudler/yip/pkg/schema"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Schema", func() {
	Context("Loading from dot notation", func() {
		oneConfigwithGarbage := map[string]interface{}{
			"stages.foo[0].name": "bar",
			"foo.bar.baz":        "foo",
			"foo.bar.bad":        "bar",
			"faz.bad[0]":         "1",
			"faz.bar[2].name":    "2",
		}
		twoConfigs := map[string]interface{}{
			"stages.foo[0].name":        "bar",
			"stages.foo[0].commands[0]": "baz",
		}

		oneConfigwithGarbageS := "stages.foo[0].name=bar boo.baz"
		twoConfigsS := "stages.foo[0].name=bar   stages.foo[0].commands[0]=baz"

		It("Reads yip file correctly", func() {
			yipConfig, err := LoadFromDotNotation(oneConfigwithGarbage)
			Expect(err).ToNot(HaveOccurred())
			Expect(yipConfig.Stages["foo"][0].Name).To(Equal("bar"))
		})
		It("Reads yip file correctly", func() {
			yipConfig, err := LoadFromDotNotation(twoConfigs)
			Expect(err).ToNot(HaveOccurred())
			Expect(yipConfig.Stages["foo"][0].Name).To(Equal("bar"))
			Expect(yipConfig.Stages["foo"][0].Commands[0]).To(Equal("baz"))
		})

		It("Reads yip file correctly", func() {
			yipConfig, err := LoadFromDotNotationS(oneConfigwithGarbageS)
			Expect(err).ToNot(HaveOccurred())
			Expect(yipConfig.Stages["foo"][0].Name).To(Equal("bar"))
		})
		It("Reads yip file correctly", func() {
			yipConfig, err := LoadFromDotNotationS(twoConfigsS)
			Expect(err).ToNot(HaveOccurred())
			Expect(yipConfig.Stages["foo"][0].Name).To(Equal("bar"))
			Expect(yipConfig.Stages["foo"][0].Commands[0]).To(Equal("baz"))
		})
	})
})
