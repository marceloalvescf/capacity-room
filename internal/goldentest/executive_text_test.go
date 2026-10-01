package goldentest

import (
	"fmt"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"unicode"

	"github.com/BurntSushi/toml"

	"github.com/marceloalvescf/k8s-capacity-report/internal/catalog"
	"github.com/marceloalvescf/k8s-capacity-report/internal/collect"
	"github.com/marceloalvescf/k8s-capacity-report/internal/discovery"
)

// originalText is one text of the original's executive tables and the
// catalog key that says the same thing. A text may hold parameters in the
// catalog's own form, {{.Name}}, with the names of the catalog entry; a
// text that two keys share is listed once for each.
type originalText struct {
	text string
	key  catalog.Key
}

// originalExecutiveTexts holds the Portuguese texts the original writes in
// the cells of the seven executive tables, letter for letter, each with
// the catalog key of the same meaning. The recorded files of
// testdata/python/ hold these texts.
var originalExecutiveTexts = func() []originalText {
	texts := []originalText{
		// The status labels.
		{"Crítica", catalog.Priority("critical")},
		{"Crítica", catalog.SectionsExecutivePoolCritical},
		{"Alta", catalog.Priority("attention")},
		{"Atenção", catalog.SectionsExecutivePoolAttention},
		{"Estável", catalog.SectionsExecutivePoolStable},
		{"Máximo do HPA atingido", catalog.SectionsExecutiveSituationHPAMax},
		{"Sem HPA", catalog.SectionsHPAsSituationNoHPA},
		{"Parcial", catalog.State(discovery.StatePartial)},
		{"Indisponível", catalog.State(discovery.StateUnavailable)},
		{"Indisponível", catalog.PlaceholderUnavailable},
		{"Indeterminado", catalog.PlaceholderUndetermined},
		{"—", catalog.PlaceholderEmptyCell},
		{"(alvo não identificado)", catalog.SectionsExecutiveTargetUnknown},
		{"Interpretação", catalog.SectionsExecutiveInterpretation},

		// The capability labels of renderers/text.py.
		{"Uso por namespace", catalog.CapabilityName("namespace_usage")},
		{"Uso por carga de trabalho", catalog.CapabilityName("workload_usage")},
		{"Solicitações e limites", catalog.CapabilityName("requests_limits")},
		{"Histórico de HPA", catalog.CapabilityName("hpa_history")},
		{"Cobertura de cargas de trabalho sem HPA", catalog.CapabilityName("workloads_without_hpa")},
		{"Associação entre endereço e nó", catalog.CapabilityName("ip_node_mapping")},
		{"Identificação de pools de nós", catalog.CapabilityName("nodepool_discovery")},
		{"Utilização dos nós", catalog.CapabilityName("node_utilization")},
		{"Escalonamento automático de nós", catalog.CapabilityName("node_autoscaling")},

		// The reason of a workload and its fragments.
		{"Solicitação e limite de CPU e memória não observados", catalog.SectionsExecutiveReasonAllMissing},
		{"CPU: pico de {{.Percent}} da solicitação", catalog.Reason("request_peak", "cpu")},
		{"Memória: pico de {{.Percent}} da solicitação", catalog.Reason("request_peak", "memory")},
		{"CPU: média de {{.Percent}} da solicitação", catalog.Reason("request_average", "cpu")},
		{"Memória: média de {{.Percent}} da solicitação", catalog.Reason("request_average", "memory")},
		{"CPU: pico de {{.Percent}} do limite", catalog.Reason("limit_peak", "cpu")},
		{"Memória: pico de {{.Percent}} do limite", catalog.Reason("limit_peak", "memory")},
		{"CPU: média de {{.Percent}} do limite", catalog.Reason("limit_average", "cpu")},
		{"Memória: média de {{.Percent}} do limite", catalog.Reason("limit_average", "memory")},
		{"CPU: pico de {{.Percent}} de utilização", catalog.Reason("usage_peak", "cpu")},
		{"Memória: pico de {{.Percent}} de utilização", catalog.Reason("usage_peak", "memory")},
		{"CPU: média de {{.Percent}} de utilização", catalog.Reason("usage_average", "cpu")},
		{"Memória: média de {{.Percent}} de utilização", catalog.Reason("usage_average", "memory")},
		{"CPU: {{.Percent}} solicitada do alocável", catalog.Reason("request_high", "cpu")},
		{"Memória: {{.Percent}} solicitada do alocável", catalog.Reason("request_high", "memory")},
		{"CPU: variação de {{.Percent}}", catalog.Reason("bursty", "cpu")},
		{"Memória: variação de {{.Percent}}", catalog.Reason("bursty", "memory")},
		{"Solicitação de CPU não observada", catalog.Reason("request_missing", "cpu")},
		{"Solicitação de Memória não observada", catalog.Reason("request_missing", "memory")},
		{"Limite de CPU não observado", catalog.Reason("limit_missing", "cpu")},
		{"Limite de Memória não observado", catalog.Reason("limit_missing", "memory")},

		// The counts of a namespace at risk.
		{"1 crítica", catalog.RiskCount("critical", 1)},
		{"{{.Count}} críticas", catalog.RiskCount("critical", 2)},
		{"1 alta", catalog.RiskCount("attention", 1)},
		{"{{.Count}} altas", catalog.RiskCount("attention", 2)},
		{"{{.Critical}} · {{.Attention}}", catalog.SectionsExecutiveRiskStatusBoth},

		// The replicas of the replica ceiling.
		{"{{.Peak}} observadas / máximo {{.Max}}", catalog.SectionsExecutiveReplicasObserved},
		{"1 ativa", catalog.SectionsExecutiveActiveReplicasSingular},
		{"{{.Count}} ativas", catalog.SectionsExecutiveActiveReplicasPlural},

		// The observations of a node pool.
		{"Uso de CPU requer atenção", catalog.FindingObservation("node_usage", "cpu")},
		{"Uso de memória requer atenção", catalog.FindingObservation("node_usage", "memory")},
		{"Solicitações de CPU elevadas", catalog.FindingObservation("node_request_high", "cpu")},
		{"Solicitações de memória elevadas", catalog.FindingObservation("node_request_high", "memory")},
		{"Solicitações elevadas com baixa utilização", catalog.FindingObservation("node_overrequested", "")},
		{"Baixa utilização média", catalog.FindingObservation("node_low_utilization", "")},
		{"Distribuição desigual de CPU", catalog.FindingObservation("nodepool_uneven", "cpu")},
		{"Distribuição desigual de memória", catalog.FindingObservation("nodepool_uneven", "memory")},
		{"Identificação do pool indisponível", catalog.FindingObservation("nodepool_unknown", "")},

		// The actions of the plan, their qualifiers, and the summaries of
		// a scope list.
		{"Revisar a solicitação de CPU e o consumo observado.", catalog.FindingAction("resource_request_usage", "cpu")},
		{"Revisar o limite de CPU e o perfil de consumo observado.", catalog.FindingAction("resource_limit_usage", "cpu")},
		{"Definir a solicitação de CPU com base no consumo observado.", catalog.FindingAction("resource_request_missing", "cpu")},
		{"Definir ou justificar o limite de CPU conforme a política da carga de trabalho.", catalog.FindingAction("resource_limit_missing", "cpu")},
		{"Revisar o dimensionamento da solicitação de CPU.", catalog.FindingAction("resource_overrequested", "cpu")},
		{"Inspecionar o perfil de rajadas de CPU antes do dimensionamento.", catalog.FindingAction("resource_bursty", "cpu")},
		{"Revisar a solicitação de memória e o consumo observado.", catalog.FindingAction("resource_request_usage", "memory")},
		{"Revisar o limite de memória e o perfil de consumo observado.", catalog.FindingAction("resource_limit_usage", "memory")},
		{"Definir a solicitação de memória com base no consumo observado.", catalog.FindingAction("resource_request_missing", "memory")},
		{"Definir ou justificar o limite de memória conforme a política da carga de trabalho.", catalog.FindingAction("resource_limit_missing", "memory")},
		{"Revisar o dimensionamento da solicitação de memória.", catalog.FindingAction("resource_overrequested", "memory")},
		{"Inspecionar o perfil de rajadas de memória antes do dimensionamento.", catalog.FindingAction("resource_bursty", "memory")},
		{"Acompanhar pressão e capacidade do nó.", catalog.FindingAction("node_usage", "cpu")},
		{"Acompanhar pressão e capacidade do nó.", catalog.FindingAction("node_usage", "memory")},
		{"Revisar solicitações e margem de agendamento.", catalog.FindingAction("node_request_high", "cpu")},
		{"Revisar solicitações e margem de agendamento.", catalog.FindingAction("node_request_high", "memory")},
		{"Revisar o dimensionamento das cargas de trabalho agendadas no nó.", catalog.FindingAction("node_overrequested", "")},
		{"Avaliar consolidação e redução segura de capacidade.", catalog.FindingAction("node_low_utilization", "")},
		{"Confirmar o pool de nós sem inferir identidade do provedor.", catalog.FindingAction("nodepool_unknown", "")},
		{"Investigar distribuição de carga e restrições de agendamento.", catalog.FindingAction("nodepool_uneven", "cpu")},
		{"Investigar distribuição de carga e restrições de agendamento.", catalog.FindingAction("nodepool_uneven", "memory")},
		{"Confirmar se o escalonamento automático é desejado para esta carga de trabalho.", catalog.FindingAction("hpa_pinned", "")},
		{"Revisar o limite máximo e a capacidade da carga de trabalho.", catalog.FindingAction("hpa_max_reached", "")},
		{"Validar a margem e o limite máximo do HPA.", catalog.FindingAction("hpa_near_max", "")},
		{"Investigar eventos, restrições de agendamento e tempo de estabilização.", catalog.FindingAction("hpa_divergence", "")},
		{"Validar se o escalonamento horizontal é necessário para este Deployment.", catalog.FindingAction("deployment_without_hpa", "")},
		{"Confirmar a estratégia de escala adequada ao StatefulSet.", catalog.FindingAction("statefulset_without_hpa", "")},
		{"Confirmar operacionalmente se o escalonamento automático do cluster está habilitado.", catalog.FindingAction("autoscaling_unknown", "")},
		{"Planejar capacidade manual e revisar a decisão de escalonamento automático.", catalog.FindingAction("autoscaling_disabled", "")},
		{"Revisar o máximo configurado e a margem do cluster.", catalog.FindingAction("autoscaling_near_max", "")},
		{"Registrar os limites operacionais sem inferir valores ausentes.", catalog.FindingAction("autoscaling_bounds_unknown", "")},
		{"Manter acompanhamento da demanda.", catalog.FindingAction("autoscaling_at_min", "")},
		{"Manter acompanhamento dos limites e eventos de escala.", catalog.FindingAction("autoscaling_enabled", "")},
		{"{{.Action}} ({{.Qualifier}})", catalog.SectionsExecutiveActionQualified},
		{"namespaces", catalog.ScopeQualifier("namespace")},
		{"cargas de trabalho", catalog.ScopeQualifier("workload")},
		{"HPAs", catalog.ScopeQualifier("hpa")},
		{"cluster", catalog.ScopeQualifier("cluster")},
		{"{{.Count}} namespaces identificados", catalog.ScopeSummary("namespace")},
		{"{{.Count}} cargas de trabalho identificadas", catalog.ScopeSummary("workload")},
		{"{{.Count}} HPAs identificados", catalog.ScopeSummary("hpa")},
		{"{{.Count}} deployments identificados", catalog.ScopeSummary("deployment")},
	}

	// The remarks of a limitation: the sentences of the warnings and of
	// the reasons of a capability.
	for code, text := range originalWarnings {
		if slices.Contains(collect.WarningCodes, code) {
			texts = append(texts, originalText{text, catalog.Warning(code)})
		}
		if slices.Contains(discovery.ReasonCodes, code) {
			texts = append(texts, originalText{text, catalog.CapabilityReason(code)})
		}
	}
	return texts
}()

// textPattern matches the texts one entry can give: the entry's key, a
// regular expression with one group for each parameter, and the names of
// the parameters in the order of the groups. A composite entry holds no
// letter outside its parameters, such as "{{.Action}} ({{.Qualifier}})",
// and matches only when every parameter is itself a text of a key.
type textPattern struct {
	key       catalog.Key
	re        *regexp.Regexp
	params    []string
	composite bool
}

// textIndex resolves a text to the catalog keys that give it.
type textIndex []textPattern

var templateParam = regexp.MustCompile(`\{\{\.([A-Za-z][A-Za-z0-9]*)\}\}`)

// newTextIndex compiles a pattern for each entry. A parameter matches any
// text without a semicolon, the separator of the fragments of one cell. An
// entry made of one parameter alone matches every text and is left out.
func newTextIndex(entries []originalText) textIndex {
	var index textIndex
	for _, e := range entries {
		var re strings.Builder
		re.WriteString("^")
		var params []string
		last := 0
		for _, m := range templateParam.FindAllStringSubmatchIndex(e.text, -1) {
			re.WriteString(regexp.QuoteMeta(e.text[last:m[0]]))
			re.WriteString(`([^;]+?)`)
			params = append(params, e.text[m[2]:m[3]])
			last = m[1]
		}
		re.WriteString(regexp.QuoteMeta(e.text[last:]))
		re.WriteString("$")
		literal := templateParam.ReplaceAllString(e.text, "")
		if literal == "" {
			continue
		}
		index = append(index, textPattern{
			key:       e.key,
			re:        regexp.MustCompile(re.String()),
			params:    params,
			composite: len(params) > 0 && !strings.ContainsFunc(literal, unicode.IsLetter),
		})
	}
	return index
}

// resolve returns every reading of text as catalog keys, sorted, or none.
// A reading is the key with its parameters, such as
// reason.request_peak.cpu(Percent="97%"); a parameter that is the text of
// a key is written as that key. A text that no entry gives whole is read
// as fragments joined by "; ", then as sentences joined by a space, when
// every fragment has a reading.
func (ix textIndex) resolve(text string) []string {
	if forms := ix.resolveWhole(text); len(forms) > 0 {
		return forms
	}
	if parts := strings.Split(text, "; "); len(parts) > 1 {
		return ix.resolveParts(parts, " ; ")
	}
	if parts := strings.SplitAfter(text, ". "); len(parts) > 1 {
		for i := range parts {
			parts[i] = strings.TrimSuffix(parts[i], " ")
		}
		return ix.resolveParts(parts, " . ")
	}
	return nil
}

// resolveParts reads each part whole and joins the readings, or returns
// none when a part has none.
func (ix textIndex) resolveParts(parts []string, joiner string) []string {
	forms := []string{""}
	for i, part := range parts {
		partForms := ix.resolveWhole(part)
		if len(partForms) == 0 {
			return nil
		}
		var next []string
		for _, prefix := range forms {
			for _, form := range partForms {
				if i == 0 {
					next = append(next, form)
				} else {
					next = append(next, prefix+joiner+form)
				}
			}
		}
		forms = next
	}
	slices.Sort(forms)
	return slices.Compact(forms)
}

// resolveWhole returns the readings of text as the text of one entry.
func (ix textIndex) resolveWhole(text string) []string {
	var forms []string
	for _, p := range ix {
		m := p.re.FindStringSubmatch(text)
		if m == nil {
			continue
		}
		type param struct {
			name  string
			forms []string
		}
		params := make([]param, 0, len(p.params))
		matched := true
		for i, name := range p.params {
			value := m[i+1]
			var valueForms []string
			if value != text {
				valueForms = ix.resolveWhole(value)
			}
			if len(valueForms) == 0 {
				if p.composite {
					matched = false
					break
				}
				valueForms = []string{strconv.Quote(value)}
			}
			params = append(params, param{name, valueForms})
		}
		if !matched {
			continue
		}
		slices.SortFunc(params, func(a, b param) int { return strings.Compare(a.name, b.name) })
		combos := []string{""}
		for i, prm := range params {
			var next []string
			for _, prefix := range combos {
				for _, form := range prm.forms {
					sep := ", "
					if i == 0 {
						sep = ""
					}
					next = append(next, prefix+sep+prm.name+"="+form)
				}
			}
			combos = next
		}
		for _, combo := range combos {
			if len(params) == 0 {
				forms = append(forms, string(p.key))
			} else {
				forms = append(forms, string(p.key)+"("+combo+")")
			}
		}
	}
	slices.Sort(forms)
	return slices.Compact(forms)
}

// catalogEntries reads every entry of the Portuguese catalog, its raw
// template text under its dotted key, from the catalog's own file.
func catalogEntries() ([]originalText, error) {
	var data map[string]any
	path := filepath.Join("..", "catalog", "locales", string(catalog.PortugueseBrazil)+".toml")
	if _, err := toml.DecodeFile(path, &data); err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	var entries []originalText
	var walk func(prefix string, node map[string]any) error
	walk = func(prefix string, node map[string]any) error {
		for name, value := range node {
			key := name
			if prefix != "" {
				key = prefix + "." + name
			}
			switch v := value.(type) {
			case string:
				entries = append(entries, originalText{v, catalog.Key(key)})
			case map[string]any:
				if err := walk(key, v); err != nil {
					return err
				}
			default:
				return fmt.Errorf("%s: entry %s is not a string", path, key)
			}
		}
		return nil
	}
	if err := walk("", data); err != nil {
		return nil, err
	}
	slices.SortFunc(entries, func(a, b originalText) int { return strings.Compare(string(a.key), string(b.key)) })
	return entries, nil
}

// executiveIndexes returns the index of the original's texts and the
// reverse index of the Portuguese catalog, built once.
var executiveIndexes = sync.OnceValues(func() ([2]textIndex, error) {
	entries, err := catalogEntries()
	if err != nil {
		return [2]textIndex{}, err
	}
	return [2]textIndex{newTextIndex(originalExecutiveTexts), newTextIndex(entries)}, nil
})

// TestOriginalExecutiveTextsNameCatalogKeys wants every key of the table
// of the original's texts to be a key of the catalog, so that a misspelt
// key cannot leave a text without a reading that matches.
func TestOriginalExecutiveTextsNameCatalogKeys(t *testing.T) {
	keys := catalog.Keys()
	for _, e := range originalExecutiveTexts {
		if !slices.Contains(keys, e.key) {
			t.Errorf("%q names %s, which the catalog does not hold", e.text, e.key)
		}
	}
}

// TestTextIndexReadsParametersAndFragments pins how a text is read: the
// parameters of an entry, a parameter that is itself the text of a key,
// fragments joined by a semicolon, and a text that no entry gives.
func TestTextIndexReadsParametersAndFragments(t *testing.T) {
	ix := newTextIndex(originalExecutiveTexts)
	tests := []struct {
		text string
		want []string
	}{
		{"CPU: pico de 97% da solicitação", []string{`reason.request_peak.cpu(Percent="97%")`}},
		{"Memória: média de 27,5% do limite", []string{`reason.limit_average.memory(Percent="27,5%")`}},
		{"Manter acompanhamento da demanda. (cluster)", []string{"sections.executive.action_qualified(Action=finding.autoscaling_at_min.action, Qualifier=scope_qualifier.cluster)"}},
		{"2 críticas · 1 alta", []string{`sections.executive.risk_status_both(Attention=risk_status.attention.singular, Critical=risk_status.critical.plural(Count="2"))`}},
		{"CPU: variação de 40%; Limite de Memória não observado", []string{`reason.bursty.cpu(Percent="40%") ; reason.limit_missing.memory`}},
		{"Indisponível", []string{"placeholder.unavailable", "state.unavailable"}},
		{"Deployment", nil},
		{"Ação qualquer (namespaces)", nil},
	}
	for _, tt := range tests {
		if got := ix.resolve(tt.text); !slices.Equal(got, tt.want) {
			t.Errorf("resolve(%q) = %q, want %q", tt.text, got, tt.want)
		}
	}
}
