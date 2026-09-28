# Graphite Report

`graphite-report` é um pacote/template Typst opinativo para relatórios corporativos genéricos. O conteúdo é escrito diretamente em Typst; o pacote aplica identidade visual monocromática, abertura compacta, metadados e paginação consistente.

## Escopo

Este README documenta o código como ele está. O que já saiu em versão e o que ainda não saiu estão separados no [CHANGELOG.md](CHANGELOG.md).

O pacote oferece:

- API pequena com `graphite-report`, `callout` e `keep-together`;
- A4 retrato, margens de 2,5 cm e Liberation Sans;
- texto justificado em português do Brasil;
- preto e cinzas neutros;
- cabeçalho ausente e número de página à direita no rodapé por padrão;
- instalação local compatível com a estrutura de pacotes do Typst;
- testes de compilação e regressão visual.

Não há conversão de Markdown, geração de DOCX, lógica para incidentes, gráficos automáticos, identidade de uma organização específica nem publicação no Typst Universe.

## Requisitos

O projeto é desenvolvido e executado em Linux.

- Typst CLI `0.15.0`;
- Python `3.11` ou mais recente, para os testes;
- checkout completo deste repositório, incluindo `assets/fonts`.

Não há mais nada. A suíte usa somente a biblioteca padrão do Python mais `pytest`; `ripgrep`,
ImageMagick e `fontconfig` deixaram de ser necessários.

Nix é uma conveniência do host, não um requisito do projeto, e por isso `flake.nix`, `flake.lock`
e `.envrc` **não são versionados**: quem clonar o repositório não os recebe, e não precisa deles.

## Início rápido no Linux

```bash
python -m venv .venv
.venv/bin/python -m pip install -e ".[test]"
```

Instale o checkout como pacote local:

```bash
.venv/bin/python scripts/install-local.py
mkdir -p build
```

Compile um exemplo usando somente as fontes do projeto:

```bash
typst compile \
  --root "$PWD" \
  --font-path assets/fonts \
  --ignore-system-fonts \
  --ignore-embedded-fonts \
  examples/minimal.typ \
  build/minimal.pdf
```

## Modelo mínimo de autoria

```typst
#import "@local/graphite-report:1.0.0": graphite-report

#show: graphite-report.with(
  title: [Relatório mensal],
)

= Resumo executivo

Escreva o conteúdo normalmente com elementos nativos do Typst.
```

Não há etapa intermediária nem conversão automática: o arquivo `.typ` é a fonte do documento. O arquivo [examples/minimal.typ](examples/minimal.typ) contém a mesma estrutura em uma versão pronta para compilar.

## API pública

O ponto de entrada exporta somente os três símbolos documentados a seguir. Módulos sob `src/` são internos.

### `graphite-report`

Uso completo:

```typst
#import "@local/graphite-report:1.0.0": (
  graphite-report,
  callout,
  keep-together,
)

#show: graphite-report.with(
  title: [Relatório mensal de operações],
  subtitle: [Resultados consolidados de julho de 2026],
  organization: [Empresa Exemplo],
  author: "Time de Operações",
  date: datetime(year: 2026, month: 7, day: 31),
  classification: [Uso interno],
  logo: none,
  outline: false,
  heading-numbering: "1.1",
  header: none,
  footer: auto,
  labels: (:),
  theme: (:),
)
```

| Parâmetro | Tipo | Padrão | Descrição |
|---|---|---|---|
| `title` | content | obrigatório | Título visível e metadado PDF. |
| `subtitle` | content ou `none` | `none` | Subtítulo da abertura compacta. |
| `organization` | content ou `none` | `none` | Organização exibida no topo. |
| `author` | string, array de strings ou `none` | `none` | Responsável visível e autor no PDF. |
| `date` | `datetime` ou `none` | `none` | Data visível e metadado PDF; `none` evita uma data corrente não determinística. |
| `classification` | content ou `none` | `none` | Classificação visível. |
| `logo` | content ou `none` | `none` | Conteúdo pronto, normalmente `image(...)`; nenhum logo é fornecido. |
| `outline` | boolean | `false` | Insere o sumário após a abertura. |
| `heading-numbering` | string ou `none` | `"1.1"` | Padrão nativo de numeração de títulos. |
| `header` | `auto`, `none` ou content | `none` | Cabeçalho nativo ou personalizado. |
| `footer` | `auto`, `none` ou content | `auto` | `auto` mostra somente o número da página à direita. |
| `labels` | dictionary | `(:)` | Sobrescritas controladas de rótulos. |
| `theme` | dictionary | `(:)` | Sobrescritas visuais controladas. |

Valores opcionais iguais a `none` não ocupam espaço na abertura.

### Rótulos

Os rótulos automáticos usam pt-BR. Todos os valores são do tipo content:

| Chave | Padrão |
|---|---|
| `author` | Responsável |
| `date` | Emissão |
| `classification` | Classificação |
| `outline` | Sumário |
| `note` | Nota |
| `info` | Informação |
| `warning` | Atenção |

Exemplo de sobrescrita manual:

```typst
labels: (
  outline: [Índice],
  author: [Elaboração],
)
```

Chaves desconhecidas geram erro. Traduzir os rótulos raramente basta sozinho — veja `lang`, `region` e `date-format` em *Tema controlado*. O pacote não promete um sistema completo de localização com catálogos por idioma.

### Tema controlado

| Chave | Tipo aceito | Padrão |
|---|---|---|
| `font` | string | `"Liberation Sans"` |
| `text-size` | length | `11pt` |
| `paper` | string | `"a4"` |
| `margin` | length, relative ou dictionary | `2.5cm` |
| `foreground` | color | preto |
| `muted` | color | 40% de luma |
| `border` | color | 75% de luma |
| `surface` | color | 95% de luma |
| `lang` | string | `"pt"` |
| `region` | string | `"BR"` |
| `date-format` | string | `"[day padding:zero]/[month padding:zero]/[year]"` |

Exemplo neutro:

```typst
theme: (
  text-size: 10.5pt,
  margin: 2.8cm,
  muted: luma(35%),
  border: luma(70%),
  surface: luma(92%),
)
```

Chaves desconhecidas geram erro. Chaves internas, espaçamentos de componentes e medidas da abertura não fazem parte da API pública.

`lang` e `region` alimentam a hifenização e as convenções tipográficas do Typst; `date-format` é passado a `datetime.display`. Traduzir apenas os rótulos deixa o texto hifenizando em português e a data em ordem pt-BR, então as três chaves andam junto com `labels`. Um relatório inteiramente em inglês:

```typst
#show: graphite-report.with(
  title: [Monthly report],
  date: datetime(year: 2026, month: 7, day: 31),
  labels: (author: [Owner], date: [Issued]),
  theme: (
    lang: "en",
    region: "US",
    date-format: "[month padding:zero]/[day padding:zero]/[year]",
  ),
)
```

### `callout`

```typst
#callout(kind: "warning", title: [Decisão necessária])[
  Confirme os responsáveis antes da publicação.
]
```

| Parâmetro | Tipo | Padrão |
|---|---|---|
| `kind` | string | `"note"` |
| `title` | content ou `none` | `none` |
| `breakable` | boolean | `false` |
| corpo final | content | obrigatório |

Os tipos aceitos em `kind` são `note`, `info` e `warning`. Eles são diferenciados por rótulo, forma e peso tipográfico, sem depender de cor. Sem `title`, o componente usa o rótulo correspondente de `labels`.

Por padrão o callout **não parte entre páginas**: se não couber no que resta da página, desce inteiro. Passe `breakable: true` para permitir a divisão, o que faz sentido em blocos longos, onde descer inteiro deixaria muito espaço vazio.

O componente só rende dentro de um documento sob `graphite-report`. Usado fora, interrompe a compilação com uma mensagem explicativa em vez de desaparecer da página.

### `keep-together`

```typst
#keep-together[
  == Próximos passos
  Este título e este conteúdo curto permanecem juntos.
]
```

O único parâmetro é o corpo final, do tipo content e obrigatório. Use-o somente para blocos pequenos. Conteúdo grande e não quebrável pode transbordar ou deixar espaço excessivo no fim da página.

## Elementos nativos

Títulos, listas, tabelas, figuras, notas de rodapé, links, rótulos, referências e `pagebreak()` continuam sendo elementos nativos do Typst.

Tabela com cabeçalho semântico e repetível:

```typst
#table(
  columns: (2fr, 1fr),
  table.header([Indicador], [Valor]),
  [Disponibilidade], [99,8%],
  [Prazo médio], [4,6 dias],
)
```

### Receita para tabelas densas

O tema estiliza a tabela na densidade do corpo do texto, que é confortável para poucas colunas e aperta quando a tabela é larga. Um relatório com muitas colunas costuma precisar de tipo menor e entrelinha mais curta, e **a densidade útil varia de tabela para tabela** — num consumidor real, cinco tabelas usam cinco tamanhos diferentes, entre 6,2 pt e 7,5 pt, cada um calibrado para o número de colunas.

Por isso o pacote não exporta um helper pronto: um valor global não serviria, e a receita abaixo tem quatorze linhas e nenhuma dependência de interno do pacote. Copie-a para o seu projeto e ajuste.

```typst
#let data-table(
  headers,
  rows,
  columns: auto,
  size: 8pt,
  inset-y: 3.5pt,
) = block(width: 100%)[
  #set text(size: size)
  #set par(justify: false, leading: 0.45em, spacing: 0.4em)
  #set table(inset: (x: 3pt, y: inset-y))
  #table(
    columns: columns,
    table.header(..headers),
    ..rows.flatten().map(item => [#item]),
  )
]

#data-table(
  ([Indicador], [Meta], [Realizado]),
  ((  [Disponibilidade], [99,5%], [99,8%]),),
  columns: (2fr, 1fr, 1fr),
  size: 7.2pt,
)
```

O tema já aplica negrito às células da primeira linha, então a receita não precisa repetir isso. O cabeçalho continua sendo `table.header`, e portanto continua repetindo a cada quebra de página.

### Receita: identificadores de máquina em tabelas

O pacote escreve em português por padrão, e o Typst aplica a convenção ortográfica do idioma: quando a quebra de linha cai sobre um hífen que já existe na palavra, o hífen é **repetido** no início da linha seguinte. Para prosa isso é correto. Para nome de recurso, de host ou de arquivo é enganoso — `aks-generalpool-29715314-vmss000001` sai da página como `aks-generalpool-29715314-` seguido de `-vmss000001`, e o leitor não tem como saber se o nome tem um hífen ou dois.

Declare o idioma **só no texto do identificador**:

```typst
#let ident(value) = text(hyphenate: false, lang: "en", value)

#table(
  columns: (3.2cm, 1fr),
  table.header([Nó], [Estado]),
  [#ident("aks-generalpool-29715314-vmss000001")], [Pronto],
)
```

A quebra continua acontecendo nos mesmos pontos, o hífen deixa de ser repetido, e o texto copiado do PDF continua sendo o nome real, byte por byte. Declarar `lang` no documento inteiro resolveria também, mas levaria junto a hifenização da prosa e o formato da data, que é justamente o que a chave `lang` do tema controla.

Três saídas que parecem servir e não servem, medidas antes de escrever esta receita:

| Tentativa | Por que não |
|---|---|
| `hyphenate: false` sozinho | governa a hifenização por dicionário, não a quebra sobre hífen que já está na cadeia; o hífen segue sendo repetido |
| `box()` em volta | não impede a quebra interna; a saída é idêntica à de não fazer nada |
| trocar `-` por `‑` (U+2011) | para a repetição porque proíbe a quebra, mas aí o identificador transborda a célula, e quem copia do PDF leva um caractere que não é o do nome |

`tests/fixtures/positive/identifier-wrapping.typ` exercita a receita e o caso sem ela lado a lado, e o golden `identifiers-1.png` prende as duas saídas.

Figura acessível e referenciável:

```typst
#figure(
  image("assets/process-flow.svg", alt: "Fluxo em três etapas"),
  caption: [Ciclo mensal de acompanhamento.],
) <process-flow>

Consulte a @process-flow.
```

## Cabeçalho e rodapé personalizados

Passe content pronto em `header` e `footer`. Ao definir um rodapé explícito, a numeração automática do `page` é substituída; inclua `counter(page).display("1")` no conteúdo personalizado.

Exemplo genérico a partir da segunda página:

```typst
#let report-header = context {
  if counter(page).get().first() > 1 {
    grid(columns: (1fr, auto), [Área responsável], [Relatório mensal])
  }
}

#let report-footer = context {
  if counter(page).get().first() > 1 {
    grid(
      columns: (1fr, auto),
      [Classificação: Uso interno],
      counter(page).display("1"),
    )
  }
}

#show: graphite-report.with(
  title: [Relatório],
  header: report-header,
  footer: report-footer,
)
```

O exemplo executável correspondente está em [examples/custom-running-elements.typ](examples/custom-running-elements.typ).

### Receita: elementos correntes a partir da segunda página

Padrão comum em relatório corporativo — a primeira página fica limpa, e da segunda em diante o cabeçalho traz a área e o logo, e o rodapé traz a classificação e o número da página.

A receita abaixo recebe o logo como parâmetro em vez de fixar um caminho, o que a mantém independente da organização e do layout de diretórios de quem a usa:

```typst
#let running-elements(logo, area: [Área responsável], classification: [Uso interno]) = (
  header: context {
    if counter(page).get().first() > 1 {
      grid(
        columns: (1fr, auto),
        column-gutter: 12pt,
        align: horizon,
        text(size: 8.5pt, weight: "bold", area),
        logo,
      )
    }
  },
  footer: context {
    if counter(page).get().first() > 1 {
      grid(
        columns: (1fr, auto),
        text(size: 8pt, [Classificação: #classification]),
        text(size: 8pt, counter(page).display("1")),
      )
    }
  },
)

#let running = running-elements(
  // O logo é conteúdo, não caminho: aqui vai um retângulo só para o exemplo
  // rodar. No seu projeto troque por
  // `image("assets/logo.svg", height: 0.8cm, alt: "Logotipo da organização")`,
  // e mantenha o `alt`, que é o que chega a tecnologia assistiva.
  rect(width: 1.6cm, height: 0.8cm, stroke: 0.5pt),
  area: [Operações],
)

#show: graphite-report.with(
  title: [Relatório],
  header: running.header,
  footer: running.footer,
)
```

A numeração não é reiniciada: a primeira página é contada, apenas não exibe o número, então a segunda mostra `2`.

**Identidade de organização não vive neste pacote.** O logo, o nome da área e a classificação pertencem ao projeto consumidor, e um preset de empresa é um módulo dele — nada aqui precisa ser importado para construí-lo, como a receita acima demonstra. O pacote não distribui marca, logo nem preset nomeado, e isso é não-objetivo declarado, não omissão.

> O conteúdo das regiões `header` e `footer` não é exposto às tecnologias assistivas pelo Typst. Informações essenciais também devem existir no corpo ou nos metadados do documento.

## Testes e regressão visual

```bash
.venv/bin/python -m pytest -q
```

Os testes de compilação recusam avisos e verificam casos positivos e diagnósticos negativos. A regressão visual compara os PNGs renderizados a 144 PPI com os golden versionados e preserva diferenças em `build/visual/diff`. Uma prova à parte confere que nenhuma página do tema contém pixel colorido, independentemente dos hashes.

Quando uma comparação falha, a suíte informa qual página divergiu. Para receber também uma imagem da diferença, instale o extra opcional:

```bash
.venv/bin/python -m pip install -e ".[test,diff]"
```

Sem ele nada deixa de ser detectado — apenas não há imagem de diferença para olhar.

Golden images nunca são alteradas por um teste. Para propor uma atualização visual:

```bash
.venv/bin/python scripts/update-golden.py
.venv/bin/python -m pytest -q tests/test_visual.py
```

Revise cada PNG gerado, página por página, antes de versioná-lo. O `scripts/update-golden.py` regrava os PNGs e o `tests/golden/SHA256SUMS` juntos, a partir da mesma declaração de snapshots que os testes leem, então as duas metades não podem divergir.

## Limitações conhecidas

- `keep-together` não mede nem divide conteúdo grande automaticamente.
- **Conteúdo que não cabe numa célula de tabela transborda sem aviso**, e pode sobrescrever a célula vizinha até ficar ilegível. O compilador não reclama: a compilação sai limpa. Cadeia longa sem espaço nem hífen é o caso mais comum, e a defesa é dimensionar a coluna — não há verificação automática. O golden `identifiers-1.png` mostra o efeito.
- Tabela pequena não resiste à quebra de página por si: se ela cair no fim da página, parte. Envolva em `keep-together` a que precisar ficar inteira.
- Cabeçalhos e rodapés não pertencem à árvore exposta a tecnologias assistivas.
- O pacote controla apenas as chaves documentadas; personalizações profundas exigem uma futura alteração de API.
- A equivalência visual depende de Typst 0.15.0 e das fontes versionadas neste repositório.
- Publicação no Typst Universe e presets corporativos nomeados são trabalhos futuros.

## Licenças

O código do projeto usa a licença MIT, disponível em [LICENSE](LICENSE).

Os arquivos Liberation Sans em `assets/fonts` usam a licença upstream preservada em [assets/fonts/LICENSE](assets/fonts/LICENSE); essa licença não substitui a licença do código do projeto.
