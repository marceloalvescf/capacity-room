package catalog

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/marceloalvescf/k8s-capacity-report/internal/discovery"
)

// fileEntries reads locales/<lang>.toml on its own, with toml.Unmarshal into
// a map[string]any and a walk of the map — not through go-i18n, and not
// through catalog.go's own parsing — so that a guard built on it can catch a
// key that go-i18n would read another way.
func fileEntries(t *testing.T, lang Language) map[Key]string {
	t.Helper()

	raw, err := fs.ReadFile(localesFS, "locales/"+string(lang)+".toml")
	if err != nil {
		t.Fatalf("reading locales/%s.toml: %v", lang, err)
	}

	var data map[string]any
	if err := toml.Unmarshal(raw, &data); err != nil {
		t.Fatalf("parsing locales/%s.toml: %v", lang, err)
	}

	entries := make(map[Key]string)
	var walk func(prefix string, node map[string]any)
	walk = func(prefix string, node map[string]any) {
		for k, v := range node {
			key := k
			if prefix != "" {
				key = prefix + "." + k
			}
			switch val := v.(type) {
			case string:
				entries[Key(key)] = val
			case map[string]any:
				walk(key, val)
			default:
				t.Fatalf("locales/%s.toml: entry %q is neither a string nor a table", lang, key)
			}
		}
	}
	walk("", data)
	return entries
}

func TestEmbeddedFilesAreValidTOML(t *testing.T) {
	for _, lang := range Languages() {
		t.Run(string(lang), func(t *testing.T) {
			entries := fileEntries(t, lang)
			if len(entries) == 0 {
				t.Fatalf("locales/%s.toml has no entries", lang)
			}
			for key, value := range entries {
				if value == "" {
					t.Errorf("locales/%s.toml: entry %q is empty", lang, key)
				}
			}
		})
	}
}

func TestKeysEqualTheFiles(t *testing.T) {
	expected := make(map[Key]bool)
	for _, k := range Keys() {
		expected[k] = true
	}

	for _, lang := range Languages() {
		t.Run(string(lang), func(t *testing.T) {
			file := fileEntries(t, lang)
			fileKeys := make(map[Key]bool, len(file))
			for k := range file {
				fileKeys[k] = true
			}

			for k := range expected {
				if !fileKeys[k] {
					t.Errorf("locales/%s.toml is missing key %q", lang, k)
				}
			}
			for k := range fileKeys {
				if !expected[k] {
					t.Errorf("locales/%s.toml has key %q, which Keys() does not list", lang, k)
				}
			}
		})
	}
}

// wordingTable holds the plan's wording table a second time, so that the
// test says the files agree with what was approved. Written by hand from
// the brief.
var wordingTable = []struct {
	key    Key
	pt, en string
}{
	{LabelsNote, "Nota", "Note"},
	{LabelsInfo, "Informação", "Information"},
	{LabelsWarning, "Atenção", "Warning"},
	{CoverTitle, "LEVANTAMENTO DE CAPACIDADE", "CAPACITY REPORT"},
	{CoverClusterLabel, "Cluster:", "Cluster:"},
	{CoverAuthorLabel, "ELABORADO POR:", "PREPARED BY:"},
	{CoverIssuedLabel, "EMITIDO EM:", "ISSUED ON:"},
	{FooterPageLabel, "Página", "Page"},
	{FrontOutlineTitle, "Sumário", "Contents"},
	{FrontMetadataHeading, "Metadados do relatório", "Report metadata"},
	{FrontMetadataHeaderField, "Campo", "Field"},
	{FrontMetadataHeaderValue, "Valor", "Value"},
	{FrontMetadataEnvironment, "Ambiente", "Environment"},
	{FrontMetadataPeriod, "Período analisado", "Analysis period"},
	{FrontMetadataPeriodRange, "{{.Start}} a {{.End}}", "{{.Start}} to {{.End}}"},
	{FrontMetadataNamespaces, "Namespaces", "Namespaces"},
	{FrontMetadataExcludedContainers, "Containers excluídos", "Excluded containers"},
	{FrontMetadataNoExcludedContainers, "Nenhum", "None"},
	{FrontMetadataOrganization, "Organização", "Organization"},
	{FrontMetadataClassification, "Classificação", "Classification"},
	{FrontLegendHeading, "Legenda de status", "Status legend"},
	{FrontLegendHeaderStatus, "Status", "Status"},
	{FrontLegendHeaderMeaning, "Interpretação", "Meaning"},
	{FrontLegendHealthy, "Saudável", "Healthy"},
	{FrontLegendAttention, "Atenção / prioridade alta", "Attention / high priority"},
	{FrontLegendCritical, "Crítico / prioridade crítica", "Critical / critical priority"},
	{FrontLegendUnavailable, "Dado indisponível", "Data unavailable"},
	{FrontLegendUndefined, "Sem base de cálculo", "No base for the ratio"},
	{FrontQualityHeading, "Qualidade dos dados", "Data quality"},
	{FrontQualityAvailable, "Disponíveis:", "Available:"},
	{FrontQualityPartial, "Parciais:", "Partial:"},
	{FrontQualityUnavailable, "Indisponíveis:", "Unavailable:"},
	{AppendixHeading, "Apêndice técnico de qualidade", "Technical quality appendix"},
	{AppendixHeaderCapability, "Capacidade", "Capability"},
	{AppendixHeaderCoverage, "Cobertura", "Coverage"},
	{AppendixHeaderSource, "Fonte", "Source"},
	{AppendixHeaderRemark, "Observação", "Remark"},
	{AppendixHeaderEvidence, "Evidência técnica", "Technical evidence"},
	{AppendixNoRemark, "Sem observação", "No remark"},
	{AppendixEmpty, "Nenhuma evidência técnica está disponível para os filtros atuais.", "No technical evidence is available for the current filters."},
	{StateAvailable, "Disponível", "Available"},
	{StatePartial, "Parcial", "Partial"},
	{StateUnavailable, "Indisponível", "Unavailable"},
	{StrategyRecordingRule, "Regra de gravação", "Recording rule"},
	{StrategyRawMetrics, "Métricas brutas", "Raw metrics"},
	{StrategyConfigured, "Configuração", "Configuration"},
	{StrategyNone, "Sem fonte", "No source"},
	{CapabilityNameNamespaceUsage, "Uso por namespace", "Usage by namespace"},
	{CapabilityNameWorkloadUsage, "Uso por carga de trabalho", "Usage by workload"},
	{CapabilityNameRequestsLimits, "Solicitações e limites", "Requests and limits"},
	{CapabilityNameHPAHistory, "Histórico de HPA", "HPA history"},
	{CapabilityNameWorkloadsWithoutHPA, "Cobertura de cargas de trabalho sem HPA", "Coverage of workloads without HPA"},
	{CapabilityNameIPNodeMapping, "Associação entre endereço e nó", "Mapping between address and node"},
	{CapabilityNameNodepoolDiscovery, "Identificação de pools de nós", "Identification of node pools"},
	{CapabilityNameNodeUtilization, "Utilização dos nós", "Node utilization"},
	{CapabilityNameNodeAutoscaling, "Escalonamento automático de nós", "Node autoscaling"},
	{CapabilityReasonMetricsNotFound, "As métricas necessárias não foram encontradas.", "The required metrics were not found."},
	{CapabilityReasonCPUUsageMetricNotFound, "A métrica de uso de CPU não foi encontrada.", "The CPU usage metric was not found."},
	{CapabilityReasonMemoryUsageMetricNotFound, "A métrica de uso de memória não foi encontrada.", "The memory usage metric was not found."},
	{CapabilityReasonUsageMetricsNotFound, "As métricas de uso de CPU e memória não foram encontradas.", "The CPU and memory usage metrics were not found."},
	{CapabilityReasonWorkloadHistoryInexact, "A associação histórica entre pods e cargas de trabalho não pôde ser reconstruída com exatidão.", "The history of which pod belonged to which workload could not be rebuilt exactly."},
	{CapabilityReasonWorkloadAssociationUnavailable, "A associação por carga de trabalho não está disponível; a cobertura está limitada a pods.", "Pods cannot be matched to workloads; coverage is limited to pods."},
	{CapabilityReasonRequestsLimitsIncomplete, "A cobertura de solicitações e limites está incompleta para CPU ou memória.", "The coverage of requests and limits is incomplete for CPU or memory."},
	{CapabilityReasonLegacyMetricsPartial, "As métricas legadas oferecem cobertura parcial de recursos.", "The legacy metrics give partial coverage of resources."},
	{CapabilityReasonHPAHistoryIncomplete, "A cobertura histórica dos HPAs está incompleta.", "The historical coverage of the HPAs is incomplete."},
	{CapabilityReasonWorkloadInventoryIncomplete, "O inventário de cargas de trabalho ou alvos de HPA está incompleto.", "The inventory of workloads or of HPA targets is incomplete."},
	{CapabilityReasonNodeAddressMappingIncomplete, "A associação entre endereços e nós pode estar incompleta.", "The mapping between addresses and nodes may be incomplete."},
	{CapabilityReasonNodepoolLabelsNotFound, "Os rótulos de pools de nós não foram encontrados.", "The node pool labels were not found."},
	{CapabilityReasonNodeAutoscalingUndetermined, "O estado do escalonamento automático de nós não pôde ser determinado.", "The state of node autoscaling could not be determined."},
}

// evidenceSubjects and evidenceEndings hold the two small tables the plan
// builds the 27 evidence sentences from, transcribed by hand from the
// brief. TestWordingMatchesThePlan crosses them to get the full sentences.
var evidenceSubjects = []struct {
	name   string
	pt, en string
}{
	{"namespace_usage", "Os dados de utilização de CPU e memória por namespace", "CPU and memory usage data by namespace"},
	{"workload_usage", "Os dados de utilização por carga de trabalho", "Workload usage data"},
	{"requests_limits", "Os dados de solicitações e limites de recursos", "Data on resource requests and limits"},
	{"hpa_history", "Os dados históricos e de configuração dos HPAs", "HPA history and configuration data"},
	{"workloads_without_hpa", "Os dados de cobertura das cargas de trabalho sem HPA", "Coverage data for workloads without HPA"},
	{"ip_node_mapping", "Os dados de associação entre endereço e nó", "Data on the mapping between address and node"},
	{"nodepool_discovery", "Os dados de identificação dos pools de nós", "Node pool identification data"},
	{"node_utilization", "Os dados de utilização dos nós", "Node utilization data"},
	{"node_autoscaling", "Os dados de escalonamento automático dos nós", "Node autoscaling data"},
}

var evidenceEndings = []struct {
	state  string
	pt, en string
}{
	{"available", "estão disponíveis para o período analisado", "is available for the analysis period"},
	{"partial", "estão disponíveis parcialmente para o período analisado", "is partly available for the analysis period"},
	{"unavailable", "não foram localizados para o período analisado", "was not found for the analysis period"},
}

func TestWordingMatchesThePlan(t *testing.T) {
	pt := fileEntries(t, PortugueseBrazil)
	en := fileEntries(t, English)

	for _, row := range wordingTable {
		if got := pt[row.key]; got != row.pt {
			t.Errorf("pt-BR %q = %q, want %q", row.key, got, row.pt)
		}
		if got := en[row.key]; got != row.en {
			t.Errorf("en %q = %q, want %q", row.key, got, row.en)
		}
	}

	for _, subject := range evidenceSubjects {
		for _, ending := range evidenceEndings {
			key := CapabilityEvidence(subject.name, ending.state)
			wantPT := subject.pt + " " + ending.pt + "."
			wantEN := subject.en + " " + ending.en + "."
			if got := pt[key]; got != wantPT {
				t.Errorf("pt-BR %q = %q, want %q", key, got, wantPT)
			}
			if got := en[key]; got != wantEN {
				t.Errorf("en %q = %q, want %q", key, got, wantEN)
			}
		}
	}
}

func TestEveryRegistryCodeHasAnEntry(t *testing.T) {
	pt := fileEntries(t, PortugueseBrazil)
	en := fileEntries(t, English)

	check := func(key Key) {
		t.Helper()
		if _, ok := pt[key]; !ok {
			t.Errorf("locales/pt-BR.toml is missing %q", key)
		}
		if _, ok := en[key]; !ok {
			t.Errorf("locales/en.toml is missing %q", key)
		}
	}

	for _, name := range discovery.Names {
		check(CapabilityName(name))
	}
	for _, state := range discovery.States {
		check(State(state))
	}
	for _, strategy := range discovery.Strategies {
		check(Strategy(strategy))
	}
	for _, code := range discovery.ReasonCodes {
		check(CapabilityReason(code))
	}
}

func sameSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	set := make(map[string]bool, len(a))
	for _, v := range a {
		set[v] = true
	}
	for _, v := range b {
		if !set[v] {
			return false
		}
	}
	return true
}

func TestLanguagesUseTheSameParameters(t *testing.T) {
	pt := fileEntries(t, PortugueseBrazil)
	en := fileEntries(t, English)

	for _, key := range Keys() {
		ptParams := paramNames(pt[key])
		enParams := paramNames(en[key])
		if !sameSet(ptParams, enParams) {
			t.Errorf("key %q uses parameters %v in Portuguese and %v in English", key, ptParams, enParams)
		}
	}
}

// reservedSegments are the words go-i18n treats specially inside a message
// table: zero.one/two/few/many/other select a plural form, and
// id/description/hash/leftDelim/rightDelim configure the message itself. A
// table made only of such words is read as one message, not a group.
var reservedSegments = map[string]bool{
	"id": true, "description": true, "hash": true, "leftdelim": true,
	"rightdelim": true, "zero": true, "one": true, "two": true, "few": true,
	"many": true, "other": true,
}

func TestNoReservedSegment(t *testing.T) {
	for _, key := range Keys() {
		for _, segment := range strings.Split(string(key), ".") {
			if reservedSegments[strings.ToLower(segment)] {
				t.Errorf("key %q has segment %q, which go-i18n reserves", key, segment)
			}
		}
	}
}

func TestNoEmDashInsideAnEntry(t *testing.T) {
	for _, lang := range Languages() {
		for key, text := range fileEntries(t, lang) {
			if strings.Contains(text, "—") && strings.TrimSpace(text) != "—" {
				t.Errorf("locales/%s.toml: entry %q holds an em-dash: %q", lang, key, text)
			}
		}
	}
}

// paramStripPattern removes a parameter reference such as {{.Start}} before
// the banned-terms scan looks at a sentence's words.
var paramStripPattern = regexp.MustCompile(`\{\{.*?\}\}`)

// wordPattern is a word: a run of letters, digits, "_", and "-", so that
// "requests_limits" and "WORKLOAD_CPU_LIMIT_MISSING" are one word each and
// never match a banned term.
var wordPattern = regexp.MustCompile(`[\p{L}\p{N}_-]+`)

// bannedTermsFor returns, for lang, the words banned in any letter case,
// the phrases banned in any letter case (each phrase a sequence of
// lower-cased words), and the words banned only as written.
func bannedTermsFor(lang Language) (anyCase map[string]bool, phrases [][]string, asWritten map[string]bool) {
	asWritten = map[string]bool{"cpu": true}

	if lang == PortugueseBrazil {
		anyCase = wordSet(
			"request", "requests", "limit", "limits", "workload", "workloads",
			"right-sizing", "headroom", "autoscaling", "autoscaler", "throttling",
			"unknown", "memory",
			"the", "is", "are", "was", "were", "not", "found", "with", "from",
			"omitted", "cannot", "may", "only", "uses",
		)
		phrases = [][]string{
			{"node", "pool"}, {"node", "pools"},
			{"grupo", "de", "nós"}, {"grupos", "de", "nós"},
		}
		return anyCase, phrases, asWritten
	}

	anyCase = wordSet("nodepool", "nodepools", "não", "está", "estão", "são", "foram", "para", "uma", "dos", "das")
	phrases = [][]string{{"node", "group"}, {"node", "groups"}}
	return anyCase, phrases, asWritten
}

func wordSet(words ...string) map[string]bool {
	set := make(map[string]bool, len(words))
	for _, w := range words {
		set[w] = true
	}
	return set
}

// findBannedTerms returns every banned term found in text, once each, in
// the order first found.
func findBannedTerms(lang Language, text string) []string {
	stripped := paramStripPattern.ReplaceAllString(text, " ")
	words := wordPattern.FindAllString(stripped, -1)
	lower := make([]string, len(words))
	for i, w := range words {
		lower[i] = strings.ToLower(w)
	}

	anyCase, phrases, asWritten := bannedTermsFor(lang)

	var found []string
	seen := make(map[string]bool)
	add := func(term string) {
		if !seen[term] {
			seen[term] = true
			found = append(found, term)
		}
	}

	for i, w := range lower {
		if anyCase[w] {
			add(w)
		}
		if asWritten[words[i]] {
			add(words[i])
		}
	}

	for _, phrase := range phrases {
		n := len(phrase)
		for i := 0; i+n <= len(lower); i++ {
			match := true
			for j := 0; j < n; j++ {
				if lower[i+j] != phrase[j] {
					match = false
					break
				}
			}
			if match {
				add(strings.Join(phrase, " "))
			}
		}
	}

	return found
}

func TestBannedTermsFinder(t *testing.T) {
	tests := []struct {
		name string
		lang Language
		text string
		want []string
	}{
		{
			name: "request in any case",
			lang: PortugueseBrazil,
			text: "Revisar o request de CPU e o consumo observado.",
			want: []string{"request"},
		},
		{
			name: "cpu as written",
			lang: PortugueseBrazil,
			text: "Uso observado de cpu no nó.",
			want: []string{"cpu"},
		},
		{
			name: "the phrase grupo de nós",
			lang: PortugueseBrazil,
			text: "O grupo de nós não pôde ser determinado.",
			want: []string{"grupo de nós"},
		},
		{
			name: "english leftovers",
			lang: PortugueseBrazil,
			text: "autoscaling state is unknown",
			want: []string{"autoscaling", "is", "unknown"},
		},
		{
			name: "correctly translated request",
			lang: PortugueseBrazil,
			text: "Revisar a solicitação de CPU e o consumo observado.",
			want: nil,
		},
		{
			name: "memory limit is proper Portuguese",
			lang: PortugueseBrazil,
			text: "Definir o limite de memória.",
			want: nil,
		},
		{
			name: "identifiers are one word each",
			lang: PortugueseBrazil,
			text: "WORKLOAD_CPU_LIMIT_MISSING requests_limits report-ident",
			want: nil,
		},
		{
			name: "kubernetes names stay as they are",
			lang: PortugueseBrazil,
			text: "Risco aumentado de OOMKill no Deployment.",
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := findBannedTerms(tt.lang, tt.text)
			if !sameSet(got, tt.want) {
				t.Errorf("findBannedTerms(%q, %q) = %v, want %v", tt.lang, tt.text, got, tt.want)
			}
		})
	}
}

func TestNoBannedTerm(t *testing.T) {
	for _, lang := range Languages() {
		for key, text := range fileEntries(t, lang) {
			if found := findBannedTerms(lang, text); len(found) > 0 {
				t.Errorf("locales/%s.toml: entry %q contains banned term(s) %v: %q", lang, key, found, text)
			}
		}
	}
}

// keyDeclarations parses keys.go itself, so that a constant or a key
// function added later is covered without a list of names kept here too.
// constants holds every exported constant declared with the explicit type
// Key; functions holds every exported top-level function whose one result
// is a Key (Keys, whose result is []Key, is not one of them).
func keyDeclarations(t *testing.T) (constants, functions []string) {
	t.Helper()

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "keys.go", nil, 0)
	if err != nil {
		t.Fatalf("parsing keys.go: %v", err)
	}

	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.GenDecl:
			if d.Tok != token.CONST {
				continue
			}
			for _, spec := range d.Specs {
				vs, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				ident, ok := vs.Type.(*ast.Ident)
				if !ok || ident.Name != "Key" {
					continue
				}
				for _, name := range vs.Names {
					if name.IsExported() {
						constants = append(constants, name.Name)
					}
				}
			}
		case *ast.FuncDecl:
			if d.Recv != nil || !d.Name.IsExported() {
				continue
			}
			if d.Type.Results == nil || len(d.Type.Results.List) != 1 {
				continue
			}
			ident, ok := d.Type.Results.List[0].Type.(*ast.Ident)
			if !ok || ident.Name != "Key" {
				continue
			}
			functions = append(functions, d.Name.Name)
		}
	}
	return constants, functions
}

// documentSelectors parses every non-test Go file of internal/document and
// collects every selector of the form catalog.<Name>, once each.
func documentSelectors(t *testing.T) map[string]bool {
	t.Helper()

	dir := filepath.Join("..", "document")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading %s: %v", dir, err)
	}

	fset := token.NewFileSet()
	selectors := make(map[string]bool)
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		path := filepath.Join(dir, name)
		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatalf("parsing %s: %v", path, err)
		}
		ast.Inspect(file, func(n ast.Node) bool {
			sel, ok := n.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			ident, ok := sel.X.(*ast.Ident)
			if !ok || ident.Name != "catalog" {
				return true
			}
			selectors[sel.Sel.Name] = true
			return true
		})
	}
	return selectors
}

// TestEveryKeyIsUsed proves that internal/document, which is the only
// non-test, non-catalog reader of these keys in this part of the project,
// leaves no key of the catalog unused: every exported Key constant of
// keys.go is either called directly (catalog.CoverTitle) or reached
// through one of its key functions (a name such as CapabilityNameFoo is
// reached through catalog.CapabilityName), and every key function is
// called at least once.
func TestEveryKeyIsUsed(t *testing.T) {
	constants, functions := keyDeclarations(t)
	selectors := documentSelectors(t)

	for _, fn := range functions {
		if !selectors[fn] {
			t.Errorf("internal/document never calls the key function catalog.%s", fn)
		}
	}

	for _, name := range constants {
		if selectors[name] {
			continue
		}
		coveredByFunction := false
		for _, fn := range functions {
			if strings.HasPrefix(name, fn) {
				coveredByFunction = true
				break
			}
		}
		if !coveredByFunction {
			t.Errorf("internal/document never uses catalog.%s, directly or through a key function", name)
		}
	}
}

// TestEntriesUseThePlainFormOnly reads every entry of both embedded files,
// parsed independently of Load, and fails on the same condition Load
// itself refuses a file for: a {{...}} span that is not the plain form
// {{.Name}}. This is what turns a future mistake into a make test failure
// rather than a run-time surprise the first time such an entry is read.
func TestEntriesUseThePlainFormOnly(t *testing.T) {
	for _, lang := range Languages() {
		for key, text := range fileEntries(t, lang) {
			if span, bad := templateSpan(text); bad {
				t.Errorf("locales/%s.toml: entry %q uses a template form that is not {{.Name}}: %s", lang, key, span)
			}
		}
	}
}
