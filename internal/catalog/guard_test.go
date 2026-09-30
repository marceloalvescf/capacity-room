package catalog

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/marceloalvescf/k8s-capacity-report/internal/collect"
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

// wordingTable holds the expected Portuguese and English wording for every
// catalog key, transcribed independently of locales/*.toml, so the test
// below catches either file drifting from what was approved.
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
	{StrategyRecordingRule, "Recording rule", "Recording rule"},
	{StrategyRawMetrics, "Raw metrics", "Raw metrics"},
	{StrategyConfigured, "Configuração", "Configuration"},
	{StrategyNone, "Sem fonte", "No source"},
	{CapabilityNameNamespaceUsage, "Uso por namespace", "Usage by namespace"},
	{CapabilityNameWorkloadUsage, "Uso por workload", "Usage by workload"},
	{CapabilityNameRequestsLimits, "Requests e limits", "Requests and limits"},
	{CapabilityNameHPAHistory, "Histórico de HPA", "HPA history"},
	{CapabilityNameWorkloadsWithoutHPA, "Cobertura de workloads sem HPA", "Coverage of workloads without HPA"},
	{CapabilityNameIPNodeMapping, "Associação entre endereço e node", "Mapping between address and node"},
	{CapabilityNameNodepoolDiscovery, "Identificação de node pools", "Identification of node pools"},
	{CapabilityNameNodeUtilization, "Utilização dos nodes", "Node utilization"},
	{CapabilityNameNodeAutoscaling, "Autoscaling de nodes", "Node autoscaling"},
	{CapabilityReasonMetricsNotFound, "As métricas necessárias não foram encontradas.", "The required metrics were not found."},
	{CapabilityReasonCPUUsageMetricNotFound, "A métrica de uso de CPU não foi encontrada.", "The CPU usage metric was not found."},
	{CapabilityReasonMemoryUsageMetricNotFound, "A métrica de uso de memória não foi encontrada.", "The memory usage metric was not found."},
	{CapabilityReasonUsageMetricsNotFound, "As métricas de uso de CPU e memória não foram encontradas.", "The CPU and memory usage metrics were not found."},
	{CapabilityReasonWorkloadHistoryInexact, "A associação histórica entre pods e workloads não pôde ser reconstruída com exatidão.", "The history of which pod belonged to which workload could not be rebuilt exactly."},
	{CapabilityReasonWorkloadAssociationUnavailable, "A associação por workload não está disponível; a cobertura está limitada a pods.", "Pods cannot be matched to workloads; coverage is limited to pods."},
	{CapabilityReasonRequestsLimitsIncomplete, "A cobertura de requests e limits está incompleta para CPU ou memória.", "The coverage of requests and limits is incomplete for CPU or memory."},
	{CapabilityReasonLegacyMetricsPartial, "As métricas legadas oferecem cobertura parcial de recursos.", "The legacy metrics give partial coverage of resources."},
	{CapabilityReasonHPAHistoryIncomplete, "A cobertura histórica dos HPAs está incompleta.", "The historical coverage of the HPAs is incomplete."},
	{CapabilityReasonWorkloadInventoryIncomplete, "O inventário de workloads ou alvos de HPA está incompleto.", "The inventory of workloads or of HPA targets is incomplete."},
	{CapabilityReasonNodeAddressMappingIncomplete, "A associação entre endereços e nodes pode estar incompleta.", "The mapping between addresses and nodes may be incomplete."},
	{CapabilityReasonNodepoolLabelsNotFound, "Os labels de node pools não foram encontrados.", "The node pool labels were not found."},
	{CapabilityReasonNodeAutoscalingUndetermined, "O estado do autoscaling de nodes não pôde ser determinado.", "The state of node autoscaling could not be determined."},
}

// evidenceSubjects and evidenceEndings hold the two small tables that cross
// to build the 27 evidence sentences (nine capabilities times three
// states). TestWordingMatchesThePlan crosses them to get the full sentences.
var evidenceSubjects = []struct {
	name   string
	pt, en string
}{
	{"namespace_usage", "Os dados de utilização de CPU e memória por namespace", "CPU and memory usage data by namespace"},
	{"workload_usage", "Os dados de utilização por workload", "Workload usage data"},
	{"requests_limits", "Os dados de requests e limits de recursos", "Data on resource requests and limits"},
	{"hpa_history", "Os dados históricos e de configuração dos HPAs", "HPA history and configuration data"},
	{"workloads_without_hpa", "Os dados de cobertura dos workloads sem HPA", "Coverage data for workloads without HPA"},
	{"ip_node_mapping", "Os dados de associação entre endereço e node", "Data on the mapping between address and node"},
	{"nodepool_discovery", "Os dados de identificação dos node pools", "Node pool identification data"},
	{"node_utilization", "Os dados de utilização dos nodes", "Node utilization data"},
	{"node_autoscaling", "Os dados de autoscaling dos nodes", "Node autoscaling data"},
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

// sectionsWordingTable holds the expected Portuguese text of the sections
// "Capacidade por Namespace" and "Recursos por workload", transcribed
// independently of locales/pt-BR.toml, so a change to the file is caught
// against the approved wording.
var sectionsWordingTable = []struct {
	key Key
	pt  string
}{
	{SectionsNamespaceLabel, "Namespace:"},
	{SectionsNamespacesHeading, "Capacidade por Namespace"},
	{SectionsNamespacesEmpty, "Nenhum dado de namespace está disponível para os filtros atuais."},
	{SectionsNamespacesHeaderResource, "Recurso"},
	{SectionsNamespacesHeaderAverage, "Uso médio"},
	{SectionsNamespacesHeaderPeak, "Pico"},
	{SectionsNamespacesHeaderRequest, "Request"},
	{SectionsNamespacesHeaderLimit, "Limit"},
	{SectionsNamespacesHeaderPeakRequest, "% Pico/Request"},
	{SectionsNamespacesHeaderPeakLimit, "% Pico/Limit"},
	{SectionsNamespacesRowCPU, "CPU"},
	{SectionsNamespacesRowMemory, "Memória"},
	{SectionsWorkloadsHeading, "Recursos por workload"},
	{SectionsWorkloadsEmpty, "Nenhum dado de workload está disponível para os filtros atuais."},
	{SectionsWorkloadsHeaderWorkload, "Workload"},
	{SectionsWorkloadsHeaderPods, "Pods"},
	{SectionsWorkloadsHeaderCPUAverage, "CPU média"},
	{SectionsWorkloadsHeaderCPUPeak, "CPU pico"},
	{SectionsWorkloadsHeaderCPURequest, "% CPU/Request"},
	{SectionsWorkloadsHeaderCPULimit, "% CPU/Limit"},
	{SectionsWorkloadsHeaderMemoryAverage, "Memória média"},
	{SectionsWorkloadsHeaderMemoryPeak, "Memória pico"},
	{SectionsWorkloadsHeaderMemoryRequest, "% Memória/Request"},
	{SectionsWorkloadsHeaderMemoryLimit, "% Memória/Limit"},
	{PlaceholderUnavailable, "Indisponível"},
	{PlaceholderNoBase, "Não definido"},
	{NotesHeading, "Avisos de interpretação"},
	{NotesHeaderRemark, "Observação"},
}

// warningWordingTable and noticeWordingTable hold the expected Portuguese
// text of the warning and notice codes of the namespace and workload
// sections, transcribed independently of locales/pt-BR.toml, the same way
// sectionsWordingTable covers the first table.
var warningWordingTable = []struct {
	code string
	pt   string
}{
	{collect.WarningNamespaceUsageIncomplete, "As métricas de uso dos namespaces estão incompletas."},
	{collect.WarningNamespaceRequestsLimitsIncomplete, "As métricas de requests e limits dos namespaces estão incompletas."},
	{collect.WarningNamespaceRequestsLimitsUnavailable, "As métricas de requests e limits dos namespaces estão indisponíveis."},
	{collect.WarningWorkloadUsageIncomplete, "As métricas de uso dos workloads estão incompletas."},
	{collect.WarningWorkloadRequestsLimitsIncomplete, "As métricas de requests e limits dos workloads estão incompletas."},
	{collect.WarningWorkloadRequestsLimitsUnavailable, "As métricas de requests e limits dos workloads estão indisponíveis."},
	{collect.WarningWorkloadHistoryCurrentAssociation, "O histórico usa a associação atual entre pods ativos e workloads."},
}

var noticeWordingTable = []struct {
	code       string
	ptSingular string
	ptPlural   string
}{
	{collect.NoticeWorkloadsOmittedNoUsage,
		"1 workload deixado de fora por não ter consumo observado no período.",
		"{{.Count}} workloads deixados de fora por não terem consumo observado no período."},
	{collect.NoticeJobsOmitted,
		"1 Job foi deixado de fora do inventário de workloads; o consumo continua contabilizado no total do namespace.",
		"{{.Count}} Jobs foram deixados de fora do inventário de workloads; o consumo continua contabilizado nos totais dos namespaces."},
}

// TestPortugueseOfTheSectionsMatchesThePlan proves that the Portuguese text
// of the sections "Capacidade por Namespace" and "Recursos por workload",
// and of the warnings and notices they use, equals the wording that was
// approved, letter for letter. The wording follows the vocabulary of spec
// 9.3, which writes every technical term in English.
func TestPortugueseOfTheSectionsMatchesThePlan(t *testing.T) {
	pt := fileEntries(t, PortugueseBrazil)

	for _, row := range sectionsWordingTable {
		if got := pt[row.key]; got != row.pt {
			t.Errorf("pt-BR %q = %q, want %q", row.key, got, row.pt)
		}
	}
	for _, row := range warningWordingTable {
		key := Warning(row.code)
		if got := pt[key]; got != row.pt {
			t.Errorf("pt-BR %q = %q, want %q", key, got, row.pt)
		}
	}
	for _, row := range noticeWordingTable {
		singularKey := Notice(row.code, 1)
		if got := pt[singularKey]; got != row.ptSingular {
			t.Errorf("pt-BR %q = %q, want %q", singularKey, got, row.ptSingular)
		}
		pluralKey := Notice(row.code, 2)
		if got := pt[pluralKey]; got != row.ptPlural {
			t.Errorf("pt-BR %q = %q, want %q", pluralKey, got, row.ptPlural)
		}
	}
}

// TestEveryWarningHasItsEntry proves that every code of collect.WarningCodes
// has an entry in both languages, and that entry takes no parameter.
func TestEveryWarningHasItsEntry(t *testing.T) {
	pt := fileEntries(t, PortugueseBrazil)
	en := fileEntries(t, English)

	if len(collect.WarningCodes) != 7 {
		t.Fatalf("collect.WarningCodes has %d codes, want 7", len(collect.WarningCodes))
	}

	for _, code := range collect.WarningCodes {
		key := Warning(code)
		ptText, ok := pt[key]
		if !ok {
			t.Errorf("locales/pt-BR.toml is missing %q", key)
		}
		enText, ok := en[key]
		if !ok {
			t.Errorf("locales/en.toml is missing %q", key)
		}
		if params := paramNames(ptText); len(params) != 0 {
			t.Errorf("pt-BR %q takes parameters %v, want none", key, params)
		}
		if params := paramNames(enText); len(params) != 0 {
			t.Errorf("en %q takes parameters %v, want none", key, params)
		}
	}
}

// TestEveryNoticeHasBothEntries proves that every code of collect.NoticeCodes
// has both its entries, singular and plural, in both languages: singular
// takes no parameter, and plural takes Count.
func TestEveryNoticeHasBothEntries(t *testing.T) {
	pt := fileEntries(t, PortugueseBrazil)
	en := fileEntries(t, English)

	if len(collect.NoticeCodes) != 2 {
		t.Fatalf("collect.NoticeCodes has %d codes, want 2", len(collect.NoticeCodes))
	}

	for _, code := range collect.NoticeCodes {
		singularKey := Notice(code, 1)
		pluralKey := Notice(code, 2)

		ptSingular, ok := pt[singularKey]
		if !ok {
			t.Errorf("locales/pt-BR.toml is missing %q", singularKey)
		}
		enSingular, ok := en[singularKey]
		if !ok {
			t.Errorf("locales/en.toml is missing %q", singularKey)
		}
		if params := paramNames(ptSingular); len(params) != 0 {
			t.Errorf("pt-BR %q takes parameters %v, want none", singularKey, params)
		}
		if params := paramNames(enSingular); len(params) != 0 {
			t.Errorf("en %q takes parameters %v, want none", singularKey, params)
		}

		ptPlural, ok := pt[pluralKey]
		if !ok {
			t.Errorf("locales/pt-BR.toml is missing %q", pluralKey)
		}
		enPlural, ok := en[pluralKey]
		if !ok {
			t.Errorf("locales/en.toml is missing %q", pluralKey)
		}
		if params := paramNames(ptPlural); !sameSet(params, []string{"Count"}) {
			t.Errorf("pt-BR %q takes parameters %v, want [Count]", pluralKey, params)
		}
		if params := paramNames(enPlural); !sameSet(params, []string{"Count"}) {
			t.Errorf("en %q takes parameters %v, want [Count]", pluralKey, params)
		}
	}
}

// familyKeyProblems returns one description for every key of entries whose
// family is warning. or notice. but whose code names no member of
// warningCodes or noticeCodes: a key left behind by a registry that dropped
// a code, or a stray key that never named one to start with.
func familyKeyProblems(entries map[Key]string, warningCodes, noticeCodes []string) []string {
	var problems []string
	for key := range entries {
		value := string(key)
		switch {
		case strings.HasPrefix(value, "warning."):
			code := strings.TrimPrefix(value, "warning.")
			if !slices.Contains(warningCodes, code) {
				problems = append(problems, fmt.Sprintf("key %q names no code of collect.WarningCodes", key))
			}
		case strings.HasPrefix(value, "notice."):
			rest := strings.TrimPrefix(value, "notice.")
			match := noticeSuffixPattern.FindStringSubmatch(rest)
			if match == nil || !slices.Contains(noticeCodes, match[1]) {
				problems = append(problems, fmt.Sprintf("key %q names no code of collect.NoticeCodes", key))
			}
		}
	}
	return problems
}

// noticeSuffixPattern matches the part of a notice. key that follows the
// prefix: a code, then ".singular" or ".plural".
var noticeSuffixPattern = regexp.MustCompile(`^(.+)\.(singular|plural)$`)

// TestNoFamilyKeyWithoutACode proves familyKeyProblems reports a key of the
// family warning. or notice. that names a code in no registry: a copy of
// the real Portuguese file, with one bogus key added.
func TestNoFamilyKeyWithoutACode(t *testing.T) {
	entries := fileEntries(t, PortugueseBrazil)

	if problems := familyKeyProblems(entries, collect.WarningCodes, collect.NoticeCodes); len(problems) != 0 {
		t.Fatalf("familyKeyProblems on the real file = %v, want none", problems)
	}

	tainted := make(map[Key]string, len(entries)+1)
	for k, v := range entries {
		tainted[k] = v
	}
	tainted[Key("warning.UNKNOWN")] = "some text"

	problems := familyKeyProblems(tainted, collect.WarningCodes, collect.NoticeCodes)
	if len(problems) != 1 || !strings.Contains(problems[0], "warning.UNKNOWN") {
		t.Fatalf("familyKeyProblems = %v, want exactly one problem naming warning.UNKNOWN", problems)
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

// The invisible characters whiteSpaceProblems checks for, named by code
// point rather than written as a character in this file, so that no
// invisible character hides in the source itself.
const (
	nonBreakingSpace   rune = 0x00A0
	narrowNoBreakSpace rune = 0x202F
	zeroWidthSpace     rune = 0x200B
	zeroWidthNonJoiner rune = 0x200C
	zeroWidthJoiner    rune = 0x200D
	byteOrderMark      rune = 0xFEFF
)

// whiteSpaceFaults are the checks whiteSpaceProblems tries, in order: the
// first that matches an entry's text is the one reported for it.
var whiteSpaceFaults = []struct {
	name  string
	match func(text string) bool
}{
	{"starts with white space", func(t string) bool {
		return strings.HasPrefix(t, " ") || strings.HasPrefix(t, "\t") || strings.HasPrefix(t, "\n")
	}},
	{"ends with white space", func(t string) bool {
		return strings.HasSuffix(t, " ") || strings.HasSuffix(t, "\t") || strings.HasSuffix(t, "\n")
	}},
	{"holds two spaces in a row", func(t string) bool { return strings.Contains(t, "  ") }},
	{"holds a tab", func(t string) bool { return strings.Contains(t, "\t") }},
	{"holds a line break", func(t string) bool { return strings.ContainsAny(t, "\n\r") }},
	{"holds a non-breaking space", func(t string) bool { return strings.ContainsRune(t, nonBreakingSpace) }},
	{"holds a narrow no-break space", func(t string) bool { return strings.ContainsRune(t, narrowNoBreakSpace) }},
	{"holds a zero-width space", func(t string) bool { return strings.ContainsRune(t, zeroWidthSpace) }},
	{"holds a zero-width non-joiner", func(t string) bool { return strings.ContainsRune(t, zeroWidthNonJoiner) }},
	{"holds a zero-width joiner", func(t string) bool { return strings.ContainsRune(t, zeroWidthJoiner) }},
	{"holds a byte order mark", func(t string) bool { return strings.ContainsRune(t, byteOrderMark) }},
}

// whiteSpaceProblems returns, for each entry of entries whose text matches
// one of whiteSpaceFaults, one line naming its key and the fault, keys
// visited in sorted order so a test's output is stable. It takes entries
// directly, rather than a language, so a test can give it entries of its
// own instead of the real locale files.
func whiteSpaceProblems(entries map[Key]string) []string {
	keys := make([]Key, 0, len(entries))
	for key := range entries {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })

	var problems []string
	for _, key := range keys {
		text := entries[key]
		for _, fault := range whiteSpaceFaults {
			if fault.match(text) {
				problems = append(problems, fmt.Sprintf("%s: %s", key, fault.name))
				break
			}
		}
	}
	return problems
}

// TestNoStrayWhiteSpaceInAnEntry checks that no entry of either locale file
// holds a fault whiteSpaceProblems reports.
func TestNoStrayWhiteSpaceInAnEntry(t *testing.T) {
	for _, lang := range Languages() {
		if problems := whiteSpaceProblems(fileEntries(t, lang)); len(problems) > 0 {
			t.Errorf("locales/%s.toml: %v", lang, problems)
		}
	}
}

// TestWhiteSpaceGuardCanFail proves whiteSpaceProblems actually catches
// each of the ten faults it reports, giving it entries of its own rather
// than the real locale files.
func TestWhiteSpaceGuardCanFail(t *testing.T) {
	tests := []struct {
		name  string
		text  string
		fault string
	}{
		{"starts or ends with white space", " both ", "starts with white space"},
		{"two spaces in a row", "a  b", "holds two spaces in a row"},
		{"a tab", "a\tb", "holds a tab"},
		{"a line break", "a\nb", "holds a line break"},
		{"a non-breaking space", fmt.Sprintf("a%cb", nonBreakingSpace), "holds a non-breaking space"},
		{"a narrow no-break space", fmt.Sprintf("a%cb", narrowNoBreakSpace), "holds a narrow no-break space"},
		{"a zero-width space", fmt.Sprintf("a%cb", zeroWidthSpace), "holds a zero-width space"},
		{"a zero-width non-joiner", fmt.Sprintf("a%cb", zeroWidthNonJoiner), "holds a zero-width non-joiner"},
		{"a zero-width joiner", fmt.Sprintf("a%cb", zeroWidthJoiner), "holds a zero-width joiner"},
		{"a byte order mark", fmt.Sprintf("a%cb", byteOrderMark), "holds a byte order mark"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entries := map[Key]string{"only.entry": tt.text}
			problems := whiteSpaceProblems(entries)
			if len(problems) != 1 {
				t.Fatalf("whiteSpaceProblems() = %v, want exactly one problem", problems)
			}
			if !strings.Contains(problems[0], "only.entry") || !strings.Contains(problems[0], tt.fault) {
				t.Errorf("problems[0] = %q, want it to name only.entry and %q", problems[0], tt.fault)
			}
		})
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
// lower-cased words), and the words banned only as written. In Portuguese it
// bans the translations of the technical terms that spec 9.3 writes in
// English, the single word nodepool, the two words headroom and right-sizing
// that have Portuguese common nouns, and English function words that betray
// a pasted English sentence.
func bannedTermsFor(lang Language) (anyCase map[string]bool, phrases [][]string, asWritten map[string]bool) {
	asWritten = map[string]bool{"cpu": true}

	if lang == PortugueseBrazil {
		anyCase = wordSet(
			"solicitação", "solicitações", "limite", "limites",
			"rótulo", "rótulos", "nó", "nós", "réplica", "réplicas",
			"nodepool", "nodepools", "right-sizing", "headroom",
			"unknown", "memory",
			"the", "is", "are", "was", "were", "not", "found", "with", "from",
			"omitted", "cannot", "may", "only", "uses",
			"and", "of", "for", "by", "on", "to", "in", "an", "be", "could",
			"this", "that", "between", "without", "usage", "coverage",
		)
		phrases = [][]string{
			{"carga", "de", "trabalho"}, {"cargas", "de", "trabalho"},
			{"pool", "de", "nós"}, {"pools", "de", "nós"},
			{"grupo", "de", "nós"}, {"grupos", "de", "nós"},
			{"regra", "de", "gravação"}, {"regras", "de", "gravação"},
			{"métricas", "brutas"},
			{"escalonamento", "automático"},
			{"limitação", "de", "cpu"},
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
			name: "solicitação in any case",
			lang: PortugueseBrazil,
			text: "Revisar a Solicitação de CPU e o consumo observado.",
			want: []string{"solicitação"},
		},
		{
			name: "request is the English word the rule asks for",
			lang: PortugueseBrazil,
			text: "Revisar o request de CPU e o consumo observado.",
			want: nil,
		},
		{
			name: "limite is banned, limit is not",
			lang: PortugueseBrazil,
			text: "Definir o limite de memória. Definir o limit de memória.",
			want: []string{"limite"},
		},
		{
			name: "the phrase carga de trabalho",
			lang: PortugueseBrazil,
			text: "Recursos por carga de trabalho e por cargas de trabalho.",
			want: []string{"carga de trabalho", "cargas de trabalho"},
		},
		{
			name: "nós in every sense",
			lang: PortugueseBrazil,
			text: "Utilização dos nós.",
			want: []string{"nós"},
		},
		{
			name: "an english sentence made of allowed nouns",
			lang: PortugueseBrazil,
			text: "Coverage data for workloads without HPA",
			want: []string{"coverage", "for", "without"},
		},
		{
			name: "an english sentence with no verb",
			lang: PortugueseBrazil,
			text: "Data on resource requests and limits",
			want: []string{"on", "and"},
		},
		{
			name: "node pool is allowed and nodepool is not",
			lang: PortugueseBrazil,
			text: "Identificação de node pools; nodepool.",
			want: []string{"nodepool"},
		},
		{
			name: "the phrase escalonamento automático",
			lang: PortugueseBrazil,
			text: "Escalonamento automático de nodes.",
			want: []string{"escalonamento automático"},
		},
		{
			name: "cpu as written",
			lang: PortugueseBrazil,
			text: "Uso observado de cpu no node.",
			want: []string{"cpu"},
		},
		{
			name: "the phrase grupo de nós",
			lang: PortugueseBrazil,
			text: "O grupo de nós não pôde ser determinado.",
			want: []string{"grupo de nós", "nós"},
		},
		{
			name: "english leftovers",
			lang: PortugueseBrazil,
			text: "autoscaling state is unknown",
			want: []string{"is", "unknown"},
		},
		{
			name: "memory is a common noun and stays Portuguese",
			lang: PortugueseBrazil,
			text: "Uso de memory acima do limit.",
			want: []string{"memory"},
		},
		{
			name: "headroom keeps its Portuguese noun",
			lang: PortugueseBrazil,
			text: "Margem observada; headroom.",
			want: []string{"headroom"},
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
// constants holds, for every exported constant declared with the explicit
// type Key, its name and the string value of its literal; functions holds
// every exported top-level function whose one result is a Key (Keys, whose
// result is []Key, is not one of them).
func keyDeclarations(t *testing.T) (constants map[string]Key, functions []string) {
	t.Helper()

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "keys.go", nil, 0)
	if err != nil {
		t.Fatalf("parsing keys.go: %v", err)
	}

	constants = make(map[string]Key)
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
				for i, name := range vs.Names {
					if !name.IsExported() {
						continue
					}
					if i >= len(vs.Values) {
						t.Fatalf("keys.go: constant %s has no value expression", name.Name)
					}
					lit, ok := vs.Values[i].(*ast.BasicLit)
					if !ok || lit.Kind != token.STRING {
						t.Fatalf("keys.go: constant %s is not a plain string literal", name.Name)
					}
					value, err := strconv.Unquote(lit.Value)
					if err != nil {
						t.Fatalf("keys.go: constant %s: %v", name.Name, err)
					}
					constants[name.Name] = Key(value)
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

// scanCatalogSelectors parses every non-test Go file of dir and returns two
// sets built from every selector of the form catalog.<Name> it finds: all,
// which holds every such selector at all, and called, the subset of those
// that are the function of a call expression (catalog.<Name>(...)) rather
// than merely named (catalog.<Name> used as a value, or the right side of
// an assignment, with no call). The distinction is what tells apart a key
// function that is genuinely invoked from one that is only mentioned.
func scanCatalogSelectors(t *testing.T, dir string) (all, called map[string]bool) {
	t.Helper()

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading %s: %v", dir, err)
	}

	fset := token.NewFileSet()
	all = make(map[string]bool)
	called = make(map[string]bool)
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

		catalogSelector := func(n ast.Node) (string, bool) {
			sel, ok := n.(*ast.SelectorExpr)
			if !ok {
				return "", false
			}
			ident, ok := sel.X.(*ast.Ident)
			if !ok || ident.Name != "catalog" {
				return "", false
			}
			return sel.Sel.Name, true
		}

		ast.Inspect(file, func(n ast.Node) bool {
			if name, ok := catalogSelector(n); ok {
				all[name] = true
			}
			if call, ok := n.(*ast.CallExpr); ok {
				if name, ok := catalogSelector(call.Fun); ok {
					called[name] = true
				}
			}
			return true
		})
	}
	return all, called
}

// keyFamily is one family of keys built from a prefix and a code of a
// registry of internal/discovery, by one key function of keys.go: every
// value that starts with prefix is a member, and the code is what follows
// the prefix.
type keyFamily struct {
	prefix   string
	function string
	codes    []string
}

// keyFamilies lists every family a constant of keys.go can belong to.
// capability.evidence., the one family Keys() builds without a constant of
// its own (see Keys' own comment), is not here: nothing in keys.go ever
// declares a constant of that prefix, so no constant is ever checked
// against it.
func keyFamilies() []keyFamily {
	return []keyFamily{
		{"capability.name.", "CapabilityName", discovery.Names},
		{"capability.reason.", "CapabilityReason", discovery.ReasonCodes},
		{"state.", "State", discovery.States},
		{"strategy.", "Strategy", discovery.Strategies},
	}
}

// familyOf returns the family whose prefix matches value and the code that
// follows it, or ok=false when value belongs to no family.
func familyOf(value Key) (family keyFamily, code string, ok bool) {
	for _, f := range keyFamilies() {
		if strings.HasPrefix(string(value), f.prefix) {
			return f, strings.TrimPrefix(string(value), f.prefix), true
		}
	}
	return keyFamily{}, "", false
}

// unusedKeyProblems compares constants and functions (as keyDeclarations
// parses them from keys.go) against all and called (as scanCatalogSelectors
// scans a document-shaped folder), and returns one description for every
// key that folder never uses:
//
//   - a key function that all shows as named but called never shows as
//     called (the selector is not the function of a call expression);
//   - a constant of a family whose code is not a member of the family's
//     registry, or whose key function called does not show as called — a
//     registry-membership check alone would let a constant such as
//     StrategyBogusPlanted = "strategy.bogus" pass merely because
//     something, somewhere, calls catalog.Strategy for an unrelated,
//     genuine strategy;
//   - every other constant whose name all does not hold at all.
func unusedKeyProblems(constants map[string]Key, functions []string, all, called map[string]bool) []string {
	var problems []string

	for _, fn := range functions {
		if !called[fn] {
			problems = append(problems, fmt.Sprintf("never calls the key function catalog.%s", fn))
		}
	}

	for name, value := range constants {
		if family, code, ok := familyOf(value); ok {
			if slices.Contains(family.codes, code) && called[family.function] {
				continue
			}
			problems = append(problems, fmt.Sprintf("never uses catalog.%s, directly or through a key function", name))
			continue
		}
		if !all[name] {
			problems = append(problems, fmt.Sprintf("never uses catalog.%s, directly or through a key function", name))
		}
	}
	return problems
}

// missingRegistryConstants checks the other direction of the same table:
// every code of every family's registry must have a constant in keys.go
// whose value is that family's prefix plus that code. A registry code with
// no matching constant would let a family drift out of sync with
// discovery's own list in silence, the same way an unused constant would.
func missingRegistryConstants(constants map[string]Key) []string {
	have := make(map[Key]bool, len(constants))
	for _, v := range constants {
		have[v] = true
	}

	var problems []string
	for _, family := range keyFamilies() {
		for _, code := range family.codes {
			want := Key(family.prefix + code)
			if !have[want] {
				problems = append(problems, fmt.Sprintf("no constant of keys.go declares %q", want))
			}
		}
	}
	return problems
}

// TestEveryKeyIsUsed proves that internal/document, which is the only
// non-test, non-catalog reader of these keys in this part of the project,
// leaves no key of the catalog unused: every exported Key constant of
// keys.go is either called directly (catalog.CoverTitle) or reached
// through one of its key functions (a name such as CapabilityNameFoo is
// reached through catalog.CapabilityName, only when its own code —
// "namespace_usage" — is genuinely a member of discovery.Names), and every
// key function is called at least once. It also checks the reverse: every
// code of every registry of internal/discovery has a matching constant in
// keys.go.
func TestEveryKeyIsUsed(t *testing.T) {
	constants, functions := keyDeclarations(t)
	all, called := scanCatalogSelectors(t, filepath.Join("..", "document"))

	for _, problem := range unusedKeyProblems(constants, functions, all, called) {
		t.Errorf("internal/document %s", problem)
	}
	for _, problem := range missingRegistryConstants(constants) {
		t.Error(problem)
	}
}

// writeGoFile writes source as name.go into a fresh temporary folder and
// returns that folder's path, so a test can hand scanCatalogSelectors a
// document-shaped folder of its own without ever touching the real
// internal/document.
func writeGoFile(t *testing.T, name, source string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, name+".go")
	if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
	return dir
}

// TestKeyUsageGuardCatchesAnUnusedFamilyMember proves unusedKeyProblems
// refuses a constant whose code is not a member of its family's registry,
// even when the family's key function is called for other, genuine values
// elsewhere — the exact shape of StrategyBogusPlanted = "strategy.bogus",
// proven here against synthetic data rather than the real keys.go and
// locale files (which TestEveryKeyIsUsed already covers as they stand).
func TestKeyUsageGuardCatchesAnUnusedFamilyMember(t *testing.T) {
	constants := map[string]Key{"StrategyBogusPlanted": "strategy.bogus"}
	functions := []string{"Strategy"}
	all := map[string]bool{"Strategy": true}
	called := map[string]bool{"Strategy": true}

	problems := unusedKeyProblems(constants, functions, all, called)
	if len(problems) != 1 || !strings.Contains(problems[0], "StrategyBogusPlanted") {
		t.Fatalf("unusedKeyProblems = %v, want exactly one problem naming StrategyBogusPlanted", problems)
	}
}

// TestKeyUsageGuardCatchesAnUnusedPlainKey proves unusedKeyProblems refuses
// a constant of no family whose name no selector of the scanned folder
// names.
func TestKeyUsageGuardCatchesAnUnusedPlainKey(t *testing.T) {
	constants := map[string]Key{"BogusPlainKey": "bogus.plain_key"}

	problems := unusedKeyProblems(constants, nil, map[string]bool{}, map[string]bool{})
	if len(problems) != 1 || !strings.Contains(problems[0], "BogusPlainKey") {
		t.Fatalf("unusedKeyProblems = %v, want exactly one problem naming BogusPlainKey", problems)
	}
}

// TestKeyUsageGuardCatchesAnUncalledFunction proves scanCatalogSelectors
// itself tells apart a key function that is merely named from one that is
// called: a copy of a source file, in a temporary folder, names
// catalog.State as a plain value (never as a call), and the scan must
// report it in all but not in called — the shape unusedKeyProblems then
// refuses as "never calls the key function".
func TestKeyUsageGuardCatchesAnUncalledFunction(t *testing.T) {
	dir := writeGoFile(t, "probe", `package probe

import "github.com/marceloalvescf/k8s-capacity-report/internal/catalog"

// stateFunc is never called: a key function that is named and not called
// must not count as used.
var stateFunc = catalog.State
`)

	all, called := scanCatalogSelectors(t, dir)
	if !all["State"] {
		t.Fatalf("scanCatalogSelectors: all[%q] = false, want true", "State")
	}
	if called["State"] {
		t.Fatalf("scanCatalogSelectors: called[%q] = true, want false (never called)", "State")
	}

	problems := unusedKeyProblems(nil, []string{"State"}, all, called)
	if len(problems) != 1 || !strings.Contains(problems[0], "catalog.State") {
		t.Fatalf("unusedKeyProblems = %v, want exactly one problem naming catalog.State", problems)
	}
}

// TestKeysHoldEveryConstant proves that Keys() never drifts from the
// constants keys.go itself declares. Keys() is built from wordingKeys, a
// list written by hand in keys.go: a constant added there and left out of
// that list would be invisible to TestKeysEqualTheFiles, which only
// compares Keys() against the locale files, never against keys.go's own
// source. This guard parses keys.go independently (through keyDeclarations,
// the same parser TestEveryKeyIsUsed uses) and compares the values of its
// constants with the part of Keys() that is not made from the registries
// of discovery and collect, in both directions.
func TestKeysHoldEveryConstant(t *testing.T) {
	constants, _ := keyDeclarations(t)

	keys := make(map[Key]bool)
	for _, k := range Keys() {
		keys[k] = true
	}

	// The keys Keys() makes from the registries of discovery and collect,
	// not from a constant of keys.go: the 27
	// capability.evidence.<name>.<state> keys, the 7 warning.<code> keys,
	// and the 4 notice.<code>.<singular|plural> keys.
	derived := make(map[Key]bool)
	for _, name := range discovery.Names {
		for _, state := range discovery.States {
			derived[CapabilityEvidence(name, state)] = true
		}
	}
	for _, code := range collect.WarningCodes {
		derived[Warning(code)] = true
	}
	for _, code := range collect.NoticeCodes {
		derived[Notice(code, 1)] = true
		derived[Notice(code, 2)] = true
	}

	constantValues := make(map[Key]bool, len(constants))
	for name, value := range constants {
		constantValues[value] = true
		if !keys[value] {
			t.Errorf("keys.go declares %s = %q, which Keys() does not hold", name, value)
		}
	}

	for key := range keys {
		if derived[key] || constantValues[key] {
			continue
		}
		t.Errorf("Keys() holds %q, which no constant of keys.go declares", key)
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
