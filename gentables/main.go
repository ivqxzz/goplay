package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
)

const headerURL = "https://raw.githubusercontent.com/FD-/RPiPlay/master/lib/playfair/omg_hax.h"

type spec struct {
	name string
	u32  bool
	dim2 bool
}

var specs = []spec{
	{"z_key", false, false},
	{"x_key", false, false},
	{"t_key", false, false},
	{"table_s1", false, false},
	{"table_s2", false, false},
	{"table_s3", false, false},
	{"table_s4", false, false},
	{"table_s5", true, false},
	{"table_s6", true, false},
	{"table_s7", true, false},
	{"table_s8", true, false},
	{"table_s9", true, false},
	{"table_s10", false, false},
	{"message_key", false, true},
	{"message_iv", false, true},
}

var intRe = regexp.MustCompile(`0[xX][0-9a-fA-F]+|\d+`)

func parseInts(s string) []uint64 {
	ms := intRe.FindAllString(s, -1)
	out := make([]uint64, 0, len(ms))
	for _, m := range ms {
		var v uint64
		var err error
		if strings.HasPrefix(m, "0x") || strings.HasPrefix(m, "0X") {
			v, err = strconv.ParseUint(m[2:], 16, 64)
		} else {
			v, err = strconv.ParseUint(m, 10, 64)
		}
		if err != nil {
			panic(err)
		}
		out = append(out, v)
	}
	return out
}

func extractBlock(src, name string) string {
	re := regexp.MustCompile(
		`(?:static\s+)?(?:const\s+)?(?:unsigned\s+char|unsigned\s+long|uint8_t|uint32_t)\s+` +
			regexp.QuoteMeta(name) + `\s*(?:\[[^\]]*\]\s*)*=\s*\{`)
	loc := re.FindStringIndex(src)
	if loc == nil {
		panic("array declaration not found: " + name)
	}
	start := loc[1] - 1
	depth := 0
	for i := start; i < len(src); i++ {
		switch src[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return src[start+1 : i]
			}
		}
	}
	panic("closing brace not found for " + name)
}

func splitGroups(body string) []string {
	var groups []string
	depth, start := 0, -1
	for i := 0; i < len(body); i++ {
		switch body[i] {
		case '{':
			if depth == 0 {
				start = i + 1
			}
			depth++
		case '}':
			depth--
			if depth == 0 && start >= 0 {
				groups = append(groups, body[start:i])
				start = -1
			}
		}
	}
	return groups
}

func main() {
	var src string
	if len(os.Args) > 1 {
		b, err := os.ReadFile(os.Args[1])
		if err != nil {
			panic(err)
		}
		src = string(b)
	} else {
		resp, err := http.Get(headerURL)
		if err != nil {
			panic(err)
		}
		defer resp.Body.Close()
		b, err := io.ReadAll(resp.Body)
		if err != nil {
			panic(err)
		}
		src = string(b)
	}

	var sb strings.Builder
	sb.WriteString("// Code generated from omg_hax.h (RPiPlay). DO NOT EDIT.\n")
	sb.WriteString("package main\n\n")

	for _, sp := range specs {
		body := extractBlock(src, sp.name)

		if sp.dim2 {
			var groupInts [][]uint64
			if g := splitGroups(body); len(g) > 0 {
				for _, s := range g {
					groupInts = append(groupInts, parseInts(s))
				}
			} else {

				all := parseInts(body)
				n := len(all) / 4
				for k := 0; k < 4; k++ {
					groupInts = append(groupInts, all[k*n:(k+1)*n])
				}
			}
			total := 0
			for _, g := range groupInts {
				total += len(g)
			}
			fmt.Fprintf(os.Stderr, "%-12s %d groups, %d total\n", sp.name, len(groupInts), total)

			sb.WriteString("var " + sp.name + " = [][]byte{\n")
			for _, ints := range groupInts {
				sb.WriteString("\t{")
				for j, v := range ints {
					if j > 0 {
						sb.WriteString(", ")
					}
					fmt.Fprintf(&sb, "0x%02x", v&0xff)
				}
				sb.WriteString("},\n")
			}
			sb.WriteString("}\n\n")
			continue
		}

		ints := parseInts(body)
		fmt.Fprintf(os.Stderr, "%-12s %d values\n", sp.name, len(ints))

		if sp.u32 {
			sb.WriteString("var " + sp.name + " = []uint32{\n\t")
			for j, v := range ints {
				fmt.Fprintf(&sb, "0x%08x,", uint32(v))
				if j%8 == 7 {
					sb.WriteString("\n\t")
				} else {
					sb.WriteString(" ")
				}
			}
			sb.WriteString("\n}\n\n")
		} else {
			sb.WriteString("var " + sp.name + " = []byte{\n\t")
			for j, v := range ints {
				fmt.Fprintf(&sb, "0x%02x,", v&0xff)
				if j%16 == 15 {
					sb.WriteString("\n\t")
				} else {
					sb.WriteString(" ")
				}
			}
			sb.WriteString("\n}\n\n")
		}
	}

	if err := os.WriteFile("omg_hax_tables.go", []byte(sb.String()), 0o644); err != nil {
		panic(err)
	}
	fmt.Fprintln(os.Stderr, "OK -> omg_hax_tables.go")
}
