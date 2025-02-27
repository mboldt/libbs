/*
 * Copyright 2018-2020 the original author or authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      https://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package libbs_test

import (
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/buildpacks/libcnb"
	. "github.com/onsi/gomega"
	"github.com/paketo-buildpacks/libpak"
	"github.com/paketo-buildpacks/libpak/bard"
	"github.com/paketo-buildpacks/libpak/effect"
	"github.com/paketo-buildpacks/libpak/effect/mocks"
	sbomMocks "github.com/paketo-buildpacks/libpak/sbom/mocks"
	"github.com/sclevine/spec"
	"github.com/stretchr/testify/mock"

	"github.com/paketo-buildpacks/libbs"
)

func testApplication(t *testing.T, context spec.G, it spec.S) {
	var (
		Expect = NewWithT(t).Expect

		cache       libbs.Cache
		ctx         libcnb.BuildContext
		application libbs.Application
		executor    *mocks.Executor
		bom         *libcnb.BOM
		sbomScanner *sbomMocks.SBOMScanner
	)

	it.Before(func() {
		var err error

		ctx.Application.Path, err = ioutil.TempDir("", "application-application")
		Expect(err).NotTo(HaveOccurred())

		ctx.Layers.Path, err = ioutil.TempDir("", "application-layers")
		Expect(err).NotTo(HaveOccurred())

		cache.Path, err = ioutil.TempDir("", "application-cache")
		Expect(err).NotTo(HaveOccurred())

		bom = &libcnb.BOM{}

		artifactResolver := libbs.ArtifactResolver{
			ConfigurationResolver: libpak.ConfigurationResolver{
				Configurations: []libpak.BuildpackConfiguration{{Default: "*"}},
			},
		}

		executor = &mocks.Executor{}
		sbomScanner = &sbomMocks.SBOMScanner{}
		sbomScanner.On("ScanBuild", ctx.Application.Path, libcnb.CycloneDXJSON, libcnb.SyftJSON).Return(nil)

		application = libbs.Application{
			ApplicationPath:  ctx.Application.Path,
			Arguments:        []string{"test-argument"},
			ArtifactResolver: artifactResolver,
			Cache:            cache,
			Command:          "test-command",
			Executor:         executor,
			LayerContributor: libpak.NewLayerContributor(
				"test",
				map[string]interface{}{},
				libcnb.LayerTypes{Cache: true},
			),
			Logger:       bard.Logger{},
			BOM:          bom,
			SBOMScanner:  sbomScanner,
			BuildpackAPI: ctx.Buildpack.API,
		}
	})

	it.After(func() {
		Expect(os.RemoveAll(ctx.Application.Path)).To(Succeed())
		Expect(os.RemoveAll(ctx.Layers.Path)).To(Succeed())
		Expect(os.RemoveAll(cache.Path)).To(Succeed())
	})

	it("contributes layer", func() {
		in, err := os.Open(filepath.Join("testdata", "stub-application.jar"))
		Expect(err).NotTo(HaveOccurred())
		out, err := os.OpenFile(filepath.Join(ctx.Application.Path, "stub-application.jar"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		Expect(err).NotTo(HaveOccurred())
		_, err = io.Copy(out, in)
		Expect(err).NotTo(HaveOccurred())
		Expect(in.Close()).To(Succeed())
		Expect(out.Close()).To(Succeed())
		Expect(ioutil.WriteFile(filepath.Join(cache.Path, "test-file-1.1.1.jar"), []byte{}, 0644)).To(Succeed())

		application.Logger = bard.NewLogger(ioutil.Discard)
		executor.On("Execute", mock.Anything).Return(nil)

		layer, err := ctx.Layers.Layer("test-layer")
		Expect(err).NotTo(HaveOccurred())

		layer, err = application.Contribute(layer)

		Expect(err).NotTo(HaveOccurred())

		Expect(layer.Cache).To(BeTrue())

		e := executor.Calls[0].Arguments[0].(effect.Execution)
		Expect(e.Command).To(Equal("test-command"))
		Expect(e.Args).To(Equal([]string{"test-argument"}))
		Expect(e.Dir).To(Equal(ctx.Application.Path))
		Expect(e.Stdout).NotTo(BeNil())
		Expect(e.Stderr).NotTo(BeNil())

		Expect(filepath.Join(layer.Path, "application.zip")).To(BeARegularFile())
		Expect(filepath.Join(ctx.Application.Path, "stub-application.jar")).NotTo(BeAnExistingFile())
		Expect(filepath.Join(ctx.Application.Path, "fixture-marker")).To(BeARegularFile())

		sbomScanner.AssertCalled(t, "ScanBuild", ctx.Application.Path, libcnb.CycloneDXJSON, libcnb.SyftJSON)
		Expect(bom.Entries).To(HaveLen(0))
	})

}
