// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package report

import (
	"fmt"
	"strings"

	"golang.org/x/vulndb/internal"
	"golang.org/x/vulndb/internal/ghsa"
)

// GHSAToReport creates a Report struct from a given GHSA SecurityAdvisory and modulePath.
func GHSAToReport(sa *ghsa.SecurityAdvisory, modulePath string) *Report {
	u := sa.UpdatedAt
	r := &Report{
		Module:       modulePath,
		Description:  sa.Description,
		Published:    sa.PublishedAt,
		LastModified: &u,
		Links:        Links{Context: []string{sa.Permalink}},
	}
	var cves, ghsas []string
	for _, id := range sa.Identifiers {
		switch id.Type {
		case "CVE":
			cves = append(cves, id.Value)
		case "GHSA":
			ghsas = append(ghsas, id.Value)
		}
	}
	r.CVEs = cves
	r.GHSAs = ghsas
	if len(sa.Vulns) == 0 {
		return r
	}
	r.Package = sa.Vulns[0].Package
	r.Versions = versions(sa.Vulns[0].EarliestFixedVersion, sa.Vulns[0].VulnerableVersionRange)
	for _, v := range sa.Vulns[1:] {
		var a Additional
		a.Package = v.Package
		a.Versions = versions(v.EarliestFixedVersion, v.VulnerableVersionRange)
		r.AdditionalPackages = append(r.AdditionalPackages, a)
	}
	r.Fix()
	return r
}

// versions extracts the versions in which a vulnerability was introduced and
// fixed from a Github Security Advisory's EarliestFixedVersion and
// VulnerableVersionRange fields, and wraps them in a []VersionRange.
//
// If the vulnRange cannot be parsed, or the earliestFixed and vulnRange are
// incompatible, populate the relevant fields with a TODO for a human to handle.
func versions(earliestFixed, vulnRange string) []VersionRange {
	// Don't try to be fully general here. Handle the common cases (which, as of
	// March 2022, are the only cases), and let a person handle the others.
	items, err := parseVulnRange(vulnRange)
	if err != nil {
		return []VersionRange{{
			Introduced: fmt.Sprintf("TODO (got error %q)", err),
		}}
	}

	var intro, fixed string

	// Most common case: a single "<" item with a version that matches earliestFixed.
	if len(items) == 1 && items[0].op == "<" && items[0].version == earliestFixed {
		intro = "v0.0.0"
		fixed = "v" + earliestFixed
	}

	// Two items, one >= and one <, with the latter matching earliestFixed.
	if len(items) == 2 && items[0].op == ">=" && items[1].op == "<" && items[1].version == earliestFixed {
		intro = "v" + items[0].version
		fixed = "v" + earliestFixed
	}

	// A single "<=" item with no fixed version.
	if len(items) == 1 && items[0].op == "<=" && earliestFixed == "" {
		intro = "v0.0.0"
	}

	if intro == "" {
		intro = fmt.Sprintf("TODO (earliest fixed %q, vuln range %q)", earliestFixed, vulnRange)
	}

	// Unset intro if vuln was always present.
	if intro == "v0.0.0" {
		intro = ""
	}

	return []VersionRange{{Introduced: intro, Fixed: fixed}}
}

type vulnRangeItem struct {
	op, version string
}

// parseVulnRange splits the contents of a GitHub Security Advisory's
// VulnerableVersionRange field into separate items.
func parseVulnRange(s string) ([]vulnRangeItem, error) {
	// A GHSA vuln range is a comma-separated list of items of the form "OP VERSION"
	// where OP is one of "<", ">", "<=" or ">=" and VERSION is a semantic
	// version.
	var items []vulnRangeItem
	parts := strings.Split(s, ",")
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		before, after, found := internal.Cut(p, " ")
		if !found {
			return nil, fmt.Errorf("invalid vuln range item %q", p)
		}
		items = append(items, vulnRangeItem{strings.TrimSpace(before), strings.TrimSpace(after)})
	}
	return items, nil
}
