# AGENTS.md
Este arquivo fornece orientações para LLMs ao trabalharem com o código deste repositório.

`graphite-report` é um pacote/template Typst para relatórios corporativos genéricos e monocromáticos, escritos diretamente em Typst. Versão 1.0.0, consumido como `@local/graphite-report:1.0.0`. O README documenta a API pública; este arquivo documenta o que só aparece ao ler vários arquivos juntos, mais as regras de processo herdadas do repositório irmão `../kubernetes-report`.

**O alvo é Linux com Python**, igual ao irmão, e os dois projetos convergem para uma plataforma web de observabilidade. **Nix é conveniência deste host, não requisito do projeto**: a suíte é `pytest` e roda com Python 3.11+, `pytest` e o `typst` no `PATH`, sem Nix nenhum. Não existe suporte a Windows para execução do pacote; o único bloco Windows que sobrou está no fim desta seção.

## Ambiente e comandos

**A suíte é `pytest` e não depende de Nix** (D2). Python 3.11+, `pytest` e o `typst` 0.15.0 no
`PATH` bastam:

```bash
python -m venv .venv
.venv/bin/python -m pip install -e ".[test]"

.venv/bin/python -m pytest -q                       # a suíte inteira
.venv/bin/python -m pytest -q tests/test_visual.py  # um módulo
.venv/bin/python -m pytest -q -k diagnostics        # por expressão
```

`GRAPHITE_TYPST` escolhe outro binário; a fixture de sessão confere a versão e **reprova**, nunca
pula.

O Nix continua disponível como wrapper reprodutível deste host, rodando **a mesma** suíte — não há
segunda implementação:

```bash
GRAPHITE_ROOT=~/Projects/Typst/graphite-report
nix develop "path:$GRAPHITE_ROOT"           # devshell: typst 0.15.0 e python com pytest
nix flake check "path:$GRAPHITE_ROOT" -L    # o mesmo pytest, em sandbox limpo
```

**`flake.nix`, `flake.lock` e `.envrc` não são versionados** (D1), então a invocação usa referência
`path:`, e **sempre com caminho absoluto**. `path:.` resolve contra o diretório corrente, e shell
com `cd` antigo produz `error: path '/nix/store/…-source/flake.nix' does not exist`, que parece
flake quebrado e é só diretório errado. Dentro de worktree sob `.worktrees/` o flake não existe: o
`path:` aponta para o diretório principal.

Para iterar em um arquivo só, sem passar pela suíte, compile direto com a mesma configuração determinística:

```bash
export XDG_DATA_HOME="$PWD/build/xdg"
.venv/bin/python scripts/install-local.py --data-home "$PWD/build/xdg" >/dev/null
mkdir -p build/focused
typst compile --diagnostic-format short --root "$PWD" \
  --font-path assets/fonts --ignore-system-fonts --ignore-embedded-fonts \
  tests/fixtures/positive/minimal.typ build/focused/minimal.pdf
```

Trocar o caminho de origem cobre qualquer fixture, exemplo ou `template/main.typ`. Para um caso negativo, o mesmo comando deve **falhar** e o stderr deve conter o fragmento fixo registrado em `tests/test_diagnostics.py`.

Conferir a superfície pública exportada:

```bash
typst eval '{ import "@local/graphite-report:1.0.0" as m; dictionary(m).keys() }' --format yaml
```

Mudança visual intencional, nunca como efeito colateral de teste:

```bash
.venv/bin/python scripts/update-golden.py
.venv/bin/python -m pytest -q tests/test_visual.py
```

`update-golden.py` apaga e regrava os PNGs e o `SHA256SUMS` juntos, lendo a **mesma** declaração de snapshots que os testes leem, em `tests/harness/snapshots.py`. Ele nunca é coletado pelo `pytest`. Revise cada PNG página por página antes de commitar.

**Python tem linter desde a v1.0.0** (`D14`): `ruff`, configurado em `pyproject.toml`, com
`E`, `F`, `B`, `I`, `UP` e `SIM` e limite de 100 colunas.

```bash
.venv/bin/python -m pip install -e ".[lint]"
.venv/bin/ruff check .          # o gate
.venv/bin/ruff check --fix .    # I, UP e SIM são corrigíveis sozinhos
```

**O linter é gate próprio e não entra no `pytest`**, de propósito: pelo `D2` a suíte roda com
Python, pytest e o `typst`, sem mais nada, e amarrá-lo a ela criaria dependência dura onde havia
zero. Por isso o extra é `lint`, separado de `test`. Rode-o antes de fechar fase, junto do
`nix flake check`.

Para `.typ` continua não havendo linter. Não invente um comando que o repositório não tem.

No Windows nativo, use apenas os comandos existentes:

```powershell
py -3.11 -m venv .venv
.\.venv\Scripts\python.exe -m pip install -e ".[test]" tzdata
.\.venv\Scripts\python.exe -m pytest -q
```

### Ferramentas fora do Nix

Verificado nesta máquina: `typst 0.15.0` está no `PATH` por uma raiz de GC explícita em `~/.local/state/nix/gcroots/typst`, e sobrevive a coleta de lixo. `awk` e `sha256sum` existem; `rg`, `identify`, `compare`, `shellcheck` e `pdfinfo` **não**. **Nada disso importa mais para a suíte**, que usa só a biblioteca padrão do Python — mas importa para scripts improvisados: o `rg` visível em shell interativo é função injetada pelo Claude Code, não binário, e script com shebang próprio não a herda.

**Não existe Python de sistema nesta bancada.** O interpretador vem do devshell, que o provisiona para criar o `.venv`. Em máquina com Python de distribuição, o Nix é dispensável do começo ao fim.

### Limitações de ambiente que já custaram tempo

- **Subagentes não conseguem usar Nix; a thread principal consegue.** O sandbox do subagente nega o daemon (`/nix/var/nix/daemon-socket/socket: Operation not permitted`, e `NIX_REMOTE=local` falha no lock do banco). Nunca instrua um subagente a rodar `nix develop` ou `nix flake check`: ele trava sem produzir trabalho. **Depois do D2 isso deixou de bloquear a validação**: passe ao subagente o caminho absoluto do `.venv/bin/python` e ele roda a suíte inteira, como no repositório irmão.
- **Binários de devshell não sobrevivem ao GC, e o `.venv` morre junto.** O interpretador do ambiente virtual é um symlink para o store, então quando o `python3-env` é coletado o sintoma vira `.venv/bin/python: No such file or directory` no meio de um comando qualquer — parece caminho errado no script, é raiz de GC faltando. Aconteceu duas vezes. Existem hoje **duas raízes** em `~/.local/state/nix/gcroots/`: `typst` e `graphite-python`. Ao recriar o ambiente virtual, refaça também a raiz:

  ```bash
  nix develop "path:$GRAPHITE_ROOT" --command bash -c '
    nix-store --add-root "$HOME/.local/state/nix/gcroots/graphite-python" \
      --indirect --realise "$(readlink -f "$(type -P python3)")" >/dev/null
    python3 -m venv .venv && .venv/bin/python -m pip install -e ".[test]"
  '
  ```

  O ambiente virtual vive na raiz do repositório, coberta pelo `.gitignore`. **Não o crie em `/tmp`**: aquilo não sobrevive à virada do dia.
- **O check por referência `path:` enxerga o diretório de trabalho inteiro, inclusive o que o `.gitignore` cobre.** Referência `path:` não filtra por git. Antes do D1 valia o oposto — o check lia só a árvore rastreada e fixture nova sem `git add` era invisível. Hoje o risco inverteu: lixo do diretório entra na cópia do store. O `flake.nix` tem uma lista `excludedFromSource` que mantém `build/`, `.git/`, `.worktrees/` e `ROADMAP.md` fora do `src` dos checks; sem ela, a derivação seria reconstruída a cada execução de teste, porque o hash mudaria junto com os artefatos. Arquivo novo que precise entrar no check não pode cair nessa lista.
- **Job do Codex trava sem avisar, e trabalhar é indistinguível de travado pelo processo.** Em 2026-08-04, no repositório irmão, um despacho ficou **7 horas** vivo com **6 segundos** de CPU acumulada e **zero escrita no repositório**. Os três sinais que separam: `ps -o time` (CPU acumulada, não `%CPU` instantânea), `mtime` dos arquivos do repositório, e o crescimento do log em `~/.codex/`. **Árvore de trabalho intocada depois do teto de tempo não é "ainda lendo", é falha** — e o subagente costuma retornar antes do job, então o relatório final não chega à thread principal: julgue o diff e rode as verificações você mesmo.
- **Ao encerrar um job travado, mate a árvore inteira, do pai `node` para baixo** — matar só o `codex app-server` deixa o plugin achando que tem servidor, e o despacho seguinte morre na largada. E **nunca mate por padrão de nome**: `pkill -f codex` atinge o lançador das **outras sessões Claude** abertas. O que separa é a ascendência — job despachado em background pende de `systemd --user`; sessão viva pende do binário `claude` dela.
- **`XDG_DATA_HOME` decide onde o pacote é instalado.** Sem `--data-home`, o `scripts/install-local.py` honra `XDG_DATA_HOME` e cai em `~/.local/share` na ausência dela, mexendo na instalação real do usuário. A suíte usa `build/xdg`; comando manual deve passar `--data-home` ou exportar a variável. O instalador recusa substituir um caminho que não seja symlink, em vez de sobrescrever.

## Delegação para subagentes

Toda tarefa de código e de documentação é executada por subagente, não pela thread principal. O modelo é o **GPT-5.6 Sol** (`gpt-5.6-sol`), pelo plugin Codex já instalado: use o comando `/codex:rescue` ou o subagente `codex:codex-rescue`. Esse já é o modelo padrão em `~/.codex/config.toml`, então `--model` só é necessário para fixar a escolha contra uma mudança do arquivo global. A thread principal planeja, distribui e julga o resultado.

Correções restritas a pontuação, erro de digitação ou costura entre dois trechos ficam na thread principal: o custo da ida e volta supera o da correção. Esse piso se refere ao tamanho da edição, não ao assunto.

### Esforço de raciocínio

A configuração global define `model_reasoning_effort = "xhigh"`, padrão inadequado para a maioria das tarefas deste repositório. Escolha por tarefa com `--effort`:

- `low` ou `medium`: trabalho mecânico — renomear, mover, aplicar especificação fechada, editar documentação, propagar a versão pelos arquivos que a repetem;
- `high`: julgamento de design dentro de escopo definido — nova chave de tema, mudança em componente, alteração de layout com efeito visual;
- `xhigh`: defeito que sobreviveu a uma primeira rodada, revisão adversarial, decisão de design sob restrições conflitantes.

Nunca use `none` ou `minimal` em trabalho que será entregue.

Sobre paralelismo:

- **Tarefas que não concorrem entre si vão em paralelo**, um subagente por tarefa, despachados na mesma rodada.
- **Tarefas que concorrem entre si vão em sequência**, na ordem exigida pela dependência, com o resultado de cada uma verificado antes de despachar a seguinte.

Neste repositório, duas tarefas **concorrem** quando compartilham qualquer um destes:

- o mesmo arquivo em `src/`, `tests/` ou `examples/`;
- qualquer PNG em `tests/golden/` ou o `SHA256SUMS`;
- a tabela `NEGATIVE_CASES` em `tests/test_diagnostics.py` junto com qualquer mensagem de `panic()` em `src/`;
- `tests/harness/snapshots.py`, que a suíte visual e o `update-golden.py` leem;
- a versão `1.0.0`, repetida à mão em toda fixture e exemplo que importa pelo identificador;
- `typst.toml`, `README.md` ou este arquivo.

Na dúvida, trate como concorrente: uma sequência desnecessária custa tempo, uma sobrescrita silenciosa custa a correção inteira.

### O que não pode ser delegado

O sandbox do subagente não alcança o daemon do Nix, e enquanto o `D2` não fechar **todos os gates dependem dele**. Ficam na thread principal, sempre:

- `nix flake check "path:$GRAPHITE_ROOT" -L`;
- `./scripts/update-golden.py` e a revisão página por página dos PNGs.

Ao subagente cabe editar fonte, fixtures, exemplos e documentação, e no máximo compilar um arquivo isolado com o `typst` do `PATH`. **Subagente não pode declarar verde.** A thread principal roda os gates e julga.

### Fechamento

Depois de qualquer rodada em paralelo, a suíte completa roda uma vez sobre o resultado combinado, pela thread principal. Subagente que valida só os próprios arquivos não enxerga regressão cruzada — e aqui a regressão cruzada é barata de produzir: qualquer mudança de espaçamento, fonte ou tema altera pixel e derruba os três goldens de uma vez.

## Roadmap de correções

`ROADMAP.md`, na raiz do repositório, é o registro vivo dos achados de revisão e do estado de cada correção. Ele **não é versionado** (consta no `.gitignore`): é ferramenta de trabalho, não entrega. Se o arquivo não existir, crie-o a partir da estrutura descrita abaixo antes de iniciar qualquer correção.

Estrutura mínima:

- **Como usar** e **Legenda** de status (`FEITO`, `ABERTO`, `PARCIAL`, `ADIADO`, `BLOQUEADO`, `REMOVIDO`) e de origem (`REV` revisão de código, `PROD` execução real, `JUDGE` revisão de trabalho de agente). `REMOVIDO` é para item cujo objeto saiu do projeto: não foi corrigido, e apagar a linha faria o próximo revisor reabrir o mesmo achado.
- Uma seção por fase, cada item com ID estável, origem, descrição em uma linha e `arquivo:linha`.
- **Entrada de achados**, onde todo achado novo é registrado antes de receber fase.
- **Regras de processo aprendidas**, para não repetir erro de orquestração.

Regras de uso:

- Consulte a fase corrente e confirme que o item ainda está `ABERTO` antes de começar. Item `FEITO` traz o commit que o fechou.
- Ao concluir, marque `FEITO` e anote o commit. Item `FEITO` sem teste correspondente não conta.
- **Todo achado vai para o `ROADMAP.md`, sempre**, não importa onde apareceu: correção, revisão de documentação, leitura de código, execução real ou julgamento do trabalho de um agente. Registre em *Entrada de achados* com ID, evidência e `arquivo:linha`, e **atribua uma fase antes de encerrar a tarefa** — achado sem fase vira dívida invisível. Se nenhuma fase existente couber, crie uma; foi assim que a Fase 5 nasceu, para dois achados que estavam órfãos havia rodadas. Achado que você decide não corrigir também entra, com o status que explica a decisão.
- Achado vindo de execução real — PDF ou PNG compilado e conferido — tem precedência sobre achado de leitura de código: já provou que afeta a saída.
- IDs são estáveis e nunca reutilizados, mesmo depois de o item ser fechado.

Os planos em `docs/superpowers/plans/` **não** cumprem esse papel: estão com 0 de 66 e 0 de 9 caixas marcadas apesar de o código correspondente estar commitado, e descrevem um suporte a Windows que o projeto removeu. São registro histórico, não estado.

## Ciclo de trabalho por branch

### Antes de abrir: conferir o registro

**Leia o `ROADMAP.md` e o `CHANGELOG.md` antes de criar a branch, e confira que os dois descrevem o estado real**, não o estado que a rodada anterior pretendia deixar:

- a fase que você vai abrir está mesmo `ABERTA`, na **linha-resumo e no cabeçalho da seção**;
- os itens dela batem com o que você pretende fazer, e nenhum já foi fechado por tabela em outra fase;
- as fases anteriores estão fechadas nos dois lugares;
- o `CHANGELOG.md` reflete o que foi entregue até aqui, e a seção `[Não lançado]` não promete o que ainda não existe;
- nenhum item aberto aponta para arquivo que deixou de existir.

**Corrija o que estiver desatualizado antes de começar, não depois.** Abrir uma fase contra um registro velho faz o trabalho inteiro ser planejado sobre premissa falsa — nesta sessão a linha-resumo da Fase 1 ficou `ABERTA` depois de fechada, a seção da v0.2.0 afirmava algo que outra fase já tinha desmentido, e uma dependência registrada entre itens era falsa. Ver **B17** e **B18**.

### Abrir uma fase é criar a branch, e nada antes disso

**Nenhuma fase começa na `master`. A criação da branch é o primeiro passo da fase, não um passo que se lembra no meio.** Enquanto ela não existir, a fase não foi iniciada, e nenhum arquivo do repositório pode ser editado por causa dela.

```bash
git switch -c <tipo>/<escopo-da-fase>          # branch no diretório principal
# ou, para isolar o diretório de trabalho:
git worktree add .worktrees/<escopo> -b <tipo>/<escopo-da-fase>
```

Confirme antes de tocar em qualquer arquivo:

```bash
git branch --show-current    # tem que ser a branch da fase, nunca `master`
```

**Se a resposta for `master`, pare.** Não edite, não commite, não "adianta um item pequeno". Crie a branch primeiro.

Isso já falhou duas vezes neste repositório, nas Fases 0 e 1, registrado como **B16** no `ROADMAP.md`. Nas duas o trabalho ficou correto e mesmo assim custou: sem branch não há ponto de reversão da fase inteira, o histórico deixa de ser legível por fase, e a regra de integração abaixo perde o objeto. Nenhuma das duas foi percebida por mim — foi o responsável que viu.

A escolha entre branch simples e worktree é livre. **Worktree tem uma armadilha específica aqui**: `flake.nix`, `flake.lock` e `.envrc` não são versionados e existem apenas no diretório de trabalho principal, então dentro de `.worktrees/` não há flake, e a referência `path:` continua apontando para o repositório principal.

### Integração

**A integração é por fase, não por item.** A branch só vai para `master` quando o último item da fase fechar, por fast-forward, acompanhada de tag quando houver versão nova. Um item pronto e testado não é mergeado sozinho — espera os irmãos. Isso mantém o histórico legível por fase e faz a tag significar algo.

### Fechar a fase

Ao encerrar as tarefas planejadas, **antes do merge**, e mesmo que a fase não gere release:

1. Rode a suíte completa, não apenas os arquivos tocados.
2. **Atualize o `ROADMAP.md`**, nos quatro lugares que envelhecem juntos:
   - o **status de cada item** da fase — `FEITO` com o commit, ou `PARCIAL`, `ADIADO`, `REMOVIDO` com o porquê;
   - a **linha-resumo da fase** na tabela do topo;
   - o **cabeçalho da seção** da fase;
   - *Entrada de achados*, com tudo que apareceu no caminho, e a fase de cada um.
3. **Revise o `CHANGELOG.md`** e inclua o que a fase mudou com efeito observável, conforme a seção abaixo. Se a fase não tiver nenhum, **declare a omissão** ao entregar, em vez de deixar o silêncio decidir.
4. **Leia os dois arquivos de volta** e confira que mudaram. Fase relatada como concluída com o resumo ainda em `ABERTA` já aconteceu, e está no **B17**; substituição que não casa sai com código 0 e parece aplicada.
5. Ajuste a versão somente se houver efeito percebido por quem consome o pacote.

**Antes de trocar de branch, copie para fora do repositório todo arquivo ignorado que você editou.** Um `git checkout` para commit que ainda rastreava esse arquivo sobrescreve a sua cópia, e o merge seguinte a apaga — foi assim que o `flake.nix` de trabalho se perdeu, ver **B14**.

## Changelog e versão

`CHANGELOG.md` é versionado, diferente do `ROADMAP.md`. Segue [Keep a Changelog](https://keepachangelog.com/pt-BR/1.1.0/) e Versionamento Semântico. A `v0.1.0` é a primeira versão; a **`v1.0.0`, de 2026-08-15, congela a API** — três símbolos mais as chaves documentadas —, e mudança incompatível a partir dela exige versão maior.

Regras de escrita:

- Agrupe em `### Adicionado`, `### Alterado`, `### Obsoleto`, `### Removido`, `### Corrigido` e `### Segurança`. Inclua um subtítulo apenas quando houver itens.
- Cada item é uma frase única descrevendo o efeito observável. Sem jargão interno, sem nome de função, sem hash de commit.
- Descreva o que o consumidor percebe. "O sumário passa a aceitar título personalizado" vale mais que "adiciona chave `outline` em `default-labels`".
- Omita commits sem efeito para quem usa o pacote: ajustes de `.gitignore`, testes, documentação interna e mudanças que só habilitam outro item já listado.
- Declare as omissões ao entregar. Omissão silenciosa é indistinguível de esquecimento.

Regras de versão:

- Mergear para a master e bumpar versão são decisões independentes. Fase composta só de teste, script ou documentação é integrada sem bump — e, sem versão nova, sem tag. Tag que repete a versão anterior mente sobre o que mudou; registre a decisão no `ROADMAP.md`.
- A tag nomeia a versão presente em `typst.toml` no commit integrado, no formato `vX.Y.Z`.
- **Bump de versão toca dezenas de arquivos**, mas deixou de ser varredura manual: `tests/test_consistency.py` confere os dois manifestos, todo identificador `@local/graphite-report:` e o caminho do instalador, e **reprova nomeando o arquivo divergente**. Rode a suíte depois do bump e ela diz o que faltou. Não vale fixar a contagem aqui: ela muda a cada fixture nova, e um número velho na documentação é pior que número nenhum. O consumidor `../kubernetes-report` fixa o mesmo padrão em `kube_capacity_report/config.py` e **não** é coberto por este teste.
- Mudança em qualquer padrão visual é comportamento observável e pede MINOR, mesmo que a API não mude: o consumidor recebe um PDF diferente.

## Arquitetura

```text
consumidor .typ
  -> lib.typ            três exports; nenhuma lógica
  -> src/config.typ     normalize(): única fronteira de validação; devolve config normalizada
  -> src/layout.typ     set document/page; abertura compacta; outline; chama apply-theme
  -> src/theme.typ      set/show de elementos nativos; resolve <graphite-callout>
  -> src/components/    callout.typ, keep-together.typ
```

Três decisões estruturais não óbvias:

**Validação concentrada.** `config.normalize()` é o único lugar que valida tipos e chaves. `layout.typ` e `theme.typ` assumem config válida e nunca revalidam. Toda mensagem de erro do pacote começa com `graphite-report:` e sai de `panic()`, não de diagnóstico do compilador.

**Protocolo interno do callout.** `callout()` não renderiza nada: valida os argumentos e emite `metadata((kind, title, body))` rotulado `<graphite-callout>`. `theme.typ` tem um `show <graphite-callout>` que chama `render-callout(payload, theme, labels)` já dentro do escopo estilizado. Isso permite exportar `callout` no `lib.typ` sem que o componente conheça tema ou rótulos. Alterar o payload exige tocar os dois lados. **Consequência viva, registrada como `P1`**: fora de um documento sob `graphite-report`, o `callout` não renderiza nada e não emite diagnóstico.

**Import unidirecional.** `theme.typ` importa `components/callout.typ`; `layout.typ` importa `theme.typ`; `lib.typ` importa `config` e `layout` de forma preguiçosa, dentro do corpo da função. Nenhum módulo importa `lib.typ` de volta.

O pacote estiliza elementos nativos do Typst em vez de substituí-los. Nenhuma interpretação de conteúdo, inferência ou conversão de Markdown acontece no fluxo.

## Contrato de determinismo

Estes quatro sinalizadores andam sempre juntos, em todo script e em toda documentação; omitir um quebra a reprodutibilidade do golden:

```
--root <repo> --font-path assets/fonts --ignore-system-fonts --ignore-embedded-fonts
```

- `flake.nix` aborta a avaliação se `pkgs.typst.version != "0.15.0"`; a fixture de sessão reconfere via `typst --version` e **reprova**, nunca pula.
- Liberation Sans versionada em `assets/fonts`, quatro faces; ausência de qualquer face falha antes da compilação.
- `date` padrão `none` evita que o Typst insira a data corrente e torne o PDF dependente do tempo.
- Aviso é falha: toda compilação positiva afirma que `warning:` não aparece no stderr.
- `tests/unit/theme-valid.typ` é renderizado como prova e **nenhum pixel pode ter cor**, conferido decodificando a imagem. É o guarda contra cor acidental no tema, independente dos hashes. Cuidado: o Typst emite RGBA mesmo sem cor, então a pergunta é sobre conteúdo e não sobre codificação.

## Suíte de testes

Três camadas, todas sob `pytest`:

| Camada | Local | Contrato |
|---|---|---|
| Unit | `tests/unit/*.typ` | `#assert` sobre `normalize()`, componentes e tema; importa `src/` por caminho relativo |
| Positiva | `tests/fixtures/positive/*.typ`, `examples/*.typ`, `template/main.typ` | compila sem erro **e sem aviso**, PDF não vazio |
| Negativa | `tests/fixtures/negative/*.typ` | compila com falha **e** o stderr contém um fragmento fixo |

Duas armadilhas provadas, registradas como `T1` e `T2` no `ROADMAP.md`:

- os fragmentos de diagnóstico esperados não vivem nas fixtures — vivem na tabela `NEGATIVE_CASES` em `tests/test_diagnostics.py`. Reescrever uma mensagem de `panic()` em `src/` quebra o teste em outro arquivo, com erro que aponta para a fixture e não para a origem;
- **fixture positiva só afirma que o compilador não reclamou.** A `tests/unit/components-valid.typ` passa verde com os três callouts ausentes da página. Componente visível precisa de golden ou de asserção sobre o texto extraído.

A regressão visual cobre **treze páginas**, em dez snapshots declarados em `tests/harness/snapshots.py`, contra quinze fontes positivas compiladas. As demais são validadas apenas quanto a compilar limpo. O `T3` **fechou**, levando a cobertura de quatro páginas para dez; as três últimas vieram do `F2` (`locale-1`), do `F5` (`callout-break-1`) e da Fase 6 (`identifiers-1`), esta o único lugar da suíte que enxerga transbordo de célula.

## Acoplamentos que quebram em silêncio

**Contagem de goldens em um lugar só.** `tests/harness/snapshots.py` declara os snapshots, e tanto a suíte quanto o `update-golden.py` derivam dali quantos PNGs existem. Acrescentar um snapshot é editar aquela tupla, e nada mais.

**A versão `1.0.0` está escrita à mão em toda fixture e exemplo que importa pelo identificador.** `tests/test_consistency.py` reprova, nomeando o culpado, se algum divergir. Ver *Changelog e versão*.

**A instalação local cria symlink para a raiz do repositório**, então o pacote instalado enxerga `tests/`, `docs/` e `build/`. Não importa hoje; importa no dia da publicação, quando o conteúdo do pacote precisa ser exatamente o que o `typst.toml` declara.

## Invariantes

- Typst permanece exatamente em 0.15.0 até que compilador e goldens sejam atualizados juntos.
- `lib.typ` exporta somente `graphite-report`, `callout` e `keep-together`. Módulos sob `src/` não fazem parte da superfície suportada; só os testes unitários os importam diretamente.
- Chave desconhecida em `theme` ou `labels` é erro, por desenho. Promover chave interna a pública é mudança de API revisada, não ajuste local.
- Identidade e assets de organizações específicas pertencem ao consumidor. Nada de marca entra em `src/`, `assets/`, `examples/` ou nos padrões; especificações de branding podem existir sob `docs/`.
- Mudança na API pública exige atualizar exemplos, fixtures, README e consumidores conhecidos na mesma rodada.
- Golden nunca é atualizado como efeito colateral de teste comum, e nunca sem revisão página por página.
- Incompatibilidade entre o pacote e um consumidor é resolvida no consumidor, salvo defeito comprovado aqui.
- O alvo é Linux. Não acrescente caminho de execução para outro sistema operacional sem decisão registrada no `ROADMAP.md`.

## Estado conhecido

- **A suíte não depende mais do Nix** (`D2`, fechado em 2026-08-10). É `pytest`, provado verde com um `PATH` contendo só o ambiente virtual e o `typst`.
- **A `v1.0.0` é a versão corrente**, criada em 2026-08-15: fechou o `D14` (linter), o `C8` (guarda das receitas do README) e as duas questões de gosto, e absorveu as cinco entradas que esperavam em `[Não lançado]`. A `0.2.0` **nunca saiu**: o projeto foi direto para a 1.0.0 e o `F7` foi partido, com a metade do bump entrando aqui e a do endereço `@preview` seguindo adiada com `F8` e `F1`. **O `../kubernetes-report` já migrou**: o pin dele passou a `1.0.0` na `v1.4.1`, em 2026-08-15, e a troca foi validada em produção contra o pacote real, com compilação `exit 0` e zero aviso.
- A `v0.1.0` foi a primeira tag do projeto, criada em 2026-08-11 depois de o critério de aceitação ser reescrito (`T6`) e executado: oito verificações automáticas mais revisão página por página dos exemplos. O critério vive em `docs/superpowers/specs/2026-08-11-v0-1-0-acceptance-design.md`.
- Suporte a Windows removido em 2026-08-10: os dois scripts PowerShell apagados, README e este arquivo limpos. Os documentos em `docs/superpowers/` ainda o descrevem e ficaram como registro histórico.
- **Todas as fases fecharam** — 0 a 3, a Documentação, a 5 e a 6. **Não há item aberto em fase nenhuma.** A Fase 6 fechou em 2026-08-15 e nasceu de conferir página a página um relatório real do consumidor que compilava com exit 0 e zero aviso: ela entregou a receita de identificador de máquina no README e o 13º golden, que é **o único lugar da suíte que enxerga transbordo de célula**. O `C8` que ela deixou em *Entrada de achados* fechou na `v1.0.0`, com teste que extrai e compila cada receita do README. As Fases 0 e 1 foram integradas direto na `master`, contra a regra de branch, e isso está registrado como `B16`; da Fase 2 em diante toda fase nasceu com branch. O que resta são itens **adiados por decisão** — `F1`, `F7` e `F8`, amarrados entre si — mais a `v1.0.0` com um item só, o linter. O `F6`, validação PDF/A ou PDF/UA, foi **medido e dispensado em 2026-08-13**: o Typst 0.15.0 já emite PDF marcado por padrão, e o responsável decidiu que o projeto fica com essa linha de base sem perseguir perfil nenhum. Leia o `ROADMAP.md` antes de começar.
- **Consumidor conhecido, um só: `../kubernetes-report`**, que gera `.typ` importando o pacote e fixa o identificador em `kube_capacity_report/config.py`. É ele que a invariante de atualizar consumidores junto com a API alcança. Arquivo de relatório solto no diretório pai — como o `../examples/report-resources-*.typ` de julho — **não é consumidor**: é saída de uma execução antiga, fora da árvore git dos dois projetos, e não entra nessa invariante nem em decisão de versão.
- **Verificação contra consumidor real usa levantamento novo, nunca relatório antigo.** Sempre que a interação com o consumidor for verificada — gerar o `.typ`, compilar, conferir página a página —, gere o relatório na hora, com o `../kubernetes-report` apontado para o cluster, e compile aquele arquivo. Relatório guardado de rodada passada carrega o identificador de pacote e o comportamento do gerador da data em que nasceu: ele mede o passado, e o que falha nele costuma ser a versão velha, não defeito de hoje. A Fase 6 nasceu certa por isso — o relatório dela foi gerado no dia.
