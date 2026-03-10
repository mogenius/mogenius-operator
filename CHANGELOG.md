# [2.20.0-develop.3](https://github.com/mogenius/mogenius-operator/compare/v2.20.0-develop.2...v2.20.0-develop.3) (2026-03-10)


### Bug Fixes

* move store setup to base system init ([896ad88](https://github.com/mogenius/mogenius-operator/commit/896ad8895baa34dbc9eef6ca6047e5f88eb097a8))

# [2.20.0-develop.2](https://github.com/mogenius/mogenius-operator/compare/v2.20.0-develop.1...v2.20.0-develop.2) (2026-03-10)


### Features

* splitting system inits ([8dc0b5e](https://github.com/mogenius/mogenius-operator/commit/8dc0b5e77872dca328ee0a8cad0a2778e7f6ced7))

# [2.20.0-develop.1](https://github.com/mogenius/mogenius-operator/compare/v2.19.1...v2.20.0-develop.1) (2026-03-10)


### Bug Fixes

* adding default relabeling for instance for servicemonitor ([e0ae0b1](https://github.com/mogenius/mogenius-operator/commit/e0ae0b18754a31a4c2acec449bf2452347c008c4))
* adding selector label to valkey svc for metrics ([cf8f4be](https://github.com/mogenius/mogenius-operator/commit/cf8f4beba3586a6d875e9fc4c5495da874e388e2))
* **deps:** update module k8s.io/klog/v2 to v2.140.0 ([#835](https://github.com/mogenius/mogenius-operator/issues/835)) ([fe56a0f](https://github.com/mogenius/mogenius-operator/commit/fe56a0f7e5759a64b28ef28b1651dedd3ea71d39))
* helm schema for valkey image tag ([005bdae](https://github.com/mogenius/mogenius-operator/commit/005bdaecedaea41af855292fee3bf9949bff395a))
* helm upgrade bug fixed ([8669876](https://github.com/mogenius/mogenius-operator/commit/86698763f6325a8243d68636089b240dd2867674))


### Features

* adding valkey exporter and servicemonitor options ([50d8ef8](https://github.com/mogenius/mogenius-operator/commit/50d8ef8994e168caa967bb1d9901d6126bb99683))

## [2.19.1](https://github.com/mogenius/mogenius-operator/compare/v2.19.0...v2.19.1) (2026-03-06)


### Bug Fixes

* remove -ng suffix ([1d92245](https://github.com/mogenius/mogenius-operator/commit/1d92245c7931bc2a286fd2e165717906bbecb626))

# [2.19.0](https://github.com/mogenius/mogenius-operator/compare/v2.18.0...v2.19.0) (2026-03-06)


### Bug Fixes

* activate helm release in dev ([cea4269](https://github.com/mogenius/mogenius-operator/commit/cea4269a1c3abe3c5f1634543f0a3e79355dbc0b))
* add changelog to .releaserc ([90d68ed](https://github.com/mogenius/mogenius-operator/commit/90d68ed985ac84332fe907e6b9745aee4a0ed1d5))
* add mcp client; update ai chat system prompt ([24fbd5a](https://github.com/mogenius/mogenius-operator/commit/24fbd5ada5b4957913f345956c7b956c733f1617))
* add new ai tools pod/event logs ([1650421](https://github.com/mogenius/mogenius-operator/commit/1650421e9e2242f1134d74f723495b8798bdc00e))
* add new Dockerfile ([49f1c45](https://github.com/mogenius/mogenius-operator/commit/49f1c45beda4d330c434dee7ea9e8cf374f1122b))
* add new github workflow actions ([1896616](https://github.com/mogenius/mogenius-operator/commit/18966165bf6767b70db4fd702a0e4dfcb76dadc1))
* add new pattern get/workload/pod-logs and get/workload/pod-events ([e77c2f0](https://github.com/mogenius/mogenius-operator/commit/e77c2f04d824ccafc101d161d9c10f4745380620))
* add permissions: inherit to prepare ([e85f7ee](https://github.com/mogenius/mogenius-operator/commit/e85f7ee41cc98b88d57b669d8c98590587ff4a74))
* add release Token ([984a645](https://github.com/mogenius/mogenius-operator/commit/984a645412c27c2697ca83d47ff36b39160565af))
* add secrets inherit ([4a037c6](https://github.com/mogenius/mogenius-operator/commit/4a037c6e5e35dcead6c775afe05654c9aec918b7))
* added list to tool usage. ([ce621b9](https://github.com/mogenius/mogenius-operator/commit/ce621b9cbfb6915f816a5231b0720e64c4cf46ce))
* adds claude files ([1bed2dd](https://github.com/mogenius/mogenius-operator/commit/1bed2dd5e919279f10bf262a062f96165df44e29))
* ai chat implemented ollama; refactoring mcp server connections ([0e263c7](https://github.com/mogenius/mogenius-operator/commit/0e263c77f307a699056bbe3f46a862c085c1b9c7))
* ai chat prompt config ([d9e6cb6](https://github.com/mogenius/mogenius-operator/commit/d9e6cb625c3b4194a5335a33e3f9710dcfc11aea))
* ai chat return values ([c88da73](https://github.com/mogenius/mogenius-operator/commit/c88da7319d0a42d9305addd9504d8022f73e762c))
* ai chat tokens ([d9a651a](https://github.com/mogenius/mogenius-operator/commit/d9a651af339128ff1232719677e2b733eabf8cee))
* aI chat user role for workspace permission check ([7922566](https://github.com/mogenius/mogenius-operator/commit/79225662682f92adb49db51cd0f2ecd1f0c8abc0))
* ai send input/output/session tokens ([b11a216](https://github.com/mogenius/mogenius-operator/commit/b11a216a78530f3f886107f2e008dc12224ac8ff))
* ai tools bug fixed. ([baef5a7](https://github.com/mogenius/mogenius-operator/commit/baef5a7c8dbcb15fa64b81b91e0c97e5326d85f6))
* async write queue for websocket to reduce latency. ([11d097b](https://github.com/mogenius/mogenius-operator/commit/11d097b000299a4a0877a3d6a74d0e06216634b9))
* bump snoopy version ([ac92d47](https://github.com/mogenius/mogenius-operator/commit/ac92d478b11cbae39ce2a61b34ef90078a5fa2d6))
* cache-improvements ([e7f391e](https://github.com/mogenius/mogenius-operator/commit/e7f391ef4eb6914aa9240a929510e9332bf169d1))
* change ref: main for SemVer ([eee94b4](https://github.com/mogenius/mogenius-operator/commit/eee94b4abc4bf7b7294bbc943d6776ad1e2f7aae))
* chat token used reset after recording ([27fcb23](https://github.com/mogenius/mogenius-operator/commit/27fcb2300fe96691fc1d6392e2c398272392205a))
* chat tokens used ([76fcb2b](https://github.com/mogenius/mogenius-operator/commit/76fcb2b6900b9b9f6c8993b5b50b574910ae5742))
* datagram size calculation fixed. ([fb9b06c](https://github.com/mogenius/mogenius-operator/commit/fb9b06c7c13339dd45f8dbd1f9077b7612f2a2d6))
* deleted aitask do now emit an event to inform the UI. ([463664d](https://github.com/mogenius/mogenius-operator/commit/463664df299e9d1ec5bf6897daf88d5023dd98e1))
* **deps:** update k8s.io/utils digest to b8788ab ([#786](https://github.com/mogenius/mogenius-operator/issues/786)) ([2505584](https://github.com/mogenius/mogenius-operator/commit/250558415fc065dbaf1afa70f81a0857737d5d7f))
* **deps:** update kubernetes packages to v0.35.1 ([#788](https://github.com/mogenius/mogenius-operator/issues/788)) ([e6d89a7](https://github.com/mogenius/mogenius-operator/commit/e6d89a77b821991a17d6e80d3c7672f5a7f6ad33))
* **deps:** update kubernetes packages to v0.35.2 ([#827](https://github.com/mogenius/mogenius-operator/issues/827)) ([544109c](https://github.com/mogenius/mogenius-operator/commit/544109cbc1822a455770b48903776d3f72bbf747))
* **deps:** update module github.com/alecthomas/kong to v1.14.0 ([#780](https://github.com/mogenius/mogenius-operator/issues/780)) ([5688349](https://github.com/mogenius/mogenius-operator/commit/56883493344473e0b1e0439efa5f52f09d50197a))
* **deps:** update module github.com/anthropics/anthropic-sdk-go to v1.20.0 ([#766](https://github.com/mogenius/mogenius-operator/issues/766)) ([b3984a3](https://github.com/mogenius/mogenius-operator/commit/b3984a30f4bbdd89fe09bba80f071386e2165631))
* **deps:** update module github.com/anthropics/anthropic-sdk-go to v1.21.0 ([#777](https://github.com/mogenius/mogenius-operator/issues/777)) ([70523ce](https://github.com/mogenius/mogenius-operator/commit/70523ce5e1dda22c36e527ade9d6092f8a31836c))
* **deps:** update module github.com/anthropics/anthropic-sdk-go to v1.22.0 ([#781](https://github.com/mogenius/mogenius-operator/issues/781)) ([dc308b4](https://github.com/mogenius/mogenius-operator/commit/dc308b45cad57bac345303778e0d52636df09f13))
* **deps:** update module github.com/anthropics/anthropic-sdk-go to v1.22.1 ([#789](https://github.com/mogenius/mogenius-operator/issues/789)) ([baeafcb](https://github.com/mogenius/mogenius-operator/commit/baeafcb5f625266417b84837455d78984ccc822b))
* **deps:** update module github.com/anthropics/anthropic-sdk-go to v1.23.0 ([#797](https://github.com/mogenius/mogenius-operator/issues/797)) ([eee6235](https://github.com/mogenius/mogenius-operator/commit/eee6235f9f4f5f784e5dcf878e80188236cd1d17))
* **deps:** update module github.com/anthropics/anthropic-sdk-go to v1.24.0 ([#798](https://github.com/mogenius/mogenius-operator/issues/798)) ([7241390](https://github.com/mogenius/mogenius-operator/commit/72413907c32bb6ab1a958dd90bf737f08ecc0944))
* **deps:** update module github.com/anthropics/anthropic-sdk-go to v1.25.0 ([#800](https://github.com/mogenius/mogenius-operator/issues/800)) ([08eb702](https://github.com/mogenius/mogenius-operator/commit/08eb702270e1574911858ca7000f88d3df80d5ce))
* **deps:** update module github.com/anthropics/anthropic-sdk-go to v1.26.0 ([#804](https://github.com/mogenius/mogenius-operator/issues/804)) ([5d1c256](https://github.com/mogenius/mogenius-operator/commit/5d1c2560b8af7568ea3b233c498b2ce071f71b40))
* **deps:** update module github.com/bitnami-labs/sealed-secrets to v0.35.0 ([#791](https://github.com/mogenius/mogenius-operator/issues/791)) ([eb3fad4](https://github.com/mogenius/mogenius-operator/commit/eb3fad41985d5c0415a6457cbcf642b70bae960a))
* **deps:** update module github.com/bitnami-labs/sealed-secrets to v0.36.0 ([#813](https://github.com/mogenius/mogenius-operator/issues/813)) ([efce86d](https://github.com/mogenius/mogenius-operator/commit/efce86d6f685aa5ffb61b217538c54b57bed7eb6))
* **deps:** update module github.com/cert-manager/cert-manager to v1.19.3 ([#773](https://github.com/mogenius/mogenius-operator/issues/773)) ([f906d76](https://github.com/mogenius/mogenius-operator/commit/f906d760b8f359e4938fdcd7316e5cc3b91f8082))
* **deps:** update module github.com/cert-manager/cert-manager to v1.19.4 ([#811](https://github.com/mogenius/mogenius-operator/issues/811)) ([4d3c32d](https://github.com/mogenius/mogenius-operator/commit/4d3c32d026f6111a13c594fb7a588ea6165e6c05))
* **deps:** update module github.com/go-git/go-git/v5 to v5.16.5 ([#783](https://github.com/mogenius/mogenius-operator/issues/783)) ([1c97cad](https://github.com/mogenius/mogenius-operator/commit/1c97cad1455373a9b36ce0f6f8d9e0cde5699d49))
* **deps:** update module github.com/go-git/go-git/v5 to v5.17.0 ([#814](https://github.com/mogenius/mogenius-operator/issues/814)) ([ba2dbdd](https://github.com/mogenius/mogenius-operator/commit/ba2dbddd02ede76d6298d3d27ef37583671ef7e2))
* **deps:** update module github.com/modelcontextprotocol/go-sdk to v1.3.0 ([#792](https://github.com/mogenius/mogenius-operator/issues/792)) ([d7aa16c](https://github.com/mogenius/mogenius-operator/commit/d7aa16c48ef54a1e5d6148b64cb8ef53f0feb801))
* **deps:** update module github.com/modelcontextprotocol/go-sdk to v1.3.1 ([#799](https://github.com/mogenius/mogenius-operator/issues/799)) ([777ed7a](https://github.com/mogenius/mogenius-operator/commit/777ed7a0e05ef1214ac857ee0e7d01e28aa71d4d))
* **deps:** update module github.com/modelcontextprotocol/go-sdk to v1.4.0 ([#828](https://github.com/mogenius/mogenius-operator/issues/828)) ([5881fc5](https://github.com/mogenius/mogenius-operator/commit/5881fc574b0d1e222b98957b91fdfbf1009701fe))
* **deps:** update module github.com/ollama/ollama to v0.15.0 ([#758](https://github.com/mogenius/mogenius-operator/issues/758)) ([cd72e52](https://github.com/mogenius/mogenius-operator/commit/cd72e52e3914c6d8c4a9e0a9ed17bade84d47584))
* **deps:** update module github.com/ollama/ollama to v0.15.1 ([#759](https://github.com/mogenius/mogenius-operator/issues/759)) ([5659861](https://github.com/mogenius/mogenius-operator/commit/56598611afd59490654c1319722ca4e90c95c4cf))
* **deps:** update module github.com/ollama/ollama to v0.15.2 ([#762](https://github.com/mogenius/mogenius-operator/issues/762)) ([fa722c1](https://github.com/mogenius/mogenius-operator/commit/fa722c17732647012a8e5d412f46533ee4d5118e))
* **deps:** update module github.com/ollama/ollama to v0.15.3 ([#767](https://github.com/mogenius/mogenius-operator/issues/767)) ([33f5206](https://github.com/mogenius/mogenius-operator/commit/33f5206d442cfcf0669a9e115cbb00f4db7ed2ed))
* **deps:** update module github.com/ollama/ollama to v0.15.4 ([#769](https://github.com/mogenius/mogenius-operator/issues/769)) ([ee31195](https://github.com/mogenius/mogenius-operator/commit/ee3119576247fcaf5283bd3493980bd8e1dcbd66))
* **deps:** update module github.com/ollama/ollama to v0.15.5 ([#779](https://github.com/mogenius/mogenius-operator/issues/779)) ([4fa84c4](https://github.com/mogenius/mogenius-operator/commit/4fa84c4a566df28ff89be6271dc65380775ff66c))
* **deps:** update module github.com/ollama/ollama to v0.16.1 ([#793](https://github.com/mogenius/mogenius-operator/issues/793)) ([adfb764](https://github.com/mogenius/mogenius-operator/commit/adfb764f109ef92492f99303cdf04390cba3ded9))
* **deps:** update module github.com/ollama/ollama to v0.16.2 ([#795](https://github.com/mogenius/mogenius-operator/issues/795)) ([2fa29d4](https://github.com/mogenius/mogenius-operator/commit/2fa29d405f6ddee6d7bf09889f429a02a46e1442))
* **deps:** update module github.com/ollama/ollama to v0.16.3 ([#805](https://github.com/mogenius/mogenius-operator/issues/805)) ([9c1508b](https://github.com/mogenius/mogenius-operator/commit/9c1508befc0af361516bf7b8f87320b90aef9d12))
* **deps:** update module github.com/ollama/ollama to v0.17.0 ([#809](https://github.com/mogenius/mogenius-operator/issues/809)) ([863ebcb](https://github.com/mogenius/mogenius-operator/commit/863ebcbda4e93939ec8a56c74f9ba28009e01086))
* **deps:** update module github.com/ollama/ollama to v0.17.4 ([#826](https://github.com/mogenius/mogenius-operator/issues/826)) ([e7e4ff7](https://github.com/mogenius/mogenius-operator/commit/e7e4ff7a0c2baac0b6e0ad44b235f1eae91aeedc))
* **deps:** update module github.com/ollama/ollama to v0.17.5 ([#829](https://github.com/mogenius/mogenius-operator/issues/829)) ([2412d81](https://github.com/mogenius/mogenius-operator/commit/2412d81fd55b42987263119bbdc9a826cc9dd13d))
* **deps:** update module github.com/ollama/ollama to v0.17.6 ([#830](https://github.com/mogenius/mogenius-operator/issues/830)) ([e7a05bb](https://github.com/mogenius/mogenius-operator/commit/e7a05bbca8f298b306828a8bdf8faa28fdbee770))
* **deps:** update module github.com/ollama/ollama to v0.17.7 ([af2ba07](https://github.com/mogenius/mogenius-operator/commit/af2ba07ae69f47331bef61adb21f0c74067dabf1))
* **deps:** update module github.com/openai/openai-go/v3 to v3.17.0 ([#763](https://github.com/mogenius/mogenius-operator/issues/763)) ([4d20dd7](https://github.com/mogenius/mogenius-operator/commit/4d20dd7a8a395df7aad3097aa6e0630e78ec2dfb))
* **deps:** update module github.com/openai/openai-go/v3 to v3.18.0 ([#778](https://github.com/mogenius/mogenius-operator/issues/778)) ([a2a03b8](https://github.com/mogenius/mogenius-operator/commit/a2a03b8b86e44e984d7ac31523fad301f800d62c))
* **deps:** update module github.com/openai/openai-go/v3 to v3.22.0 ([#794](https://github.com/mogenius/mogenius-operator/issues/794)) ([19fa14f](https://github.com/mogenius/mogenius-operator/commit/19fa14f06b0ff45d97ef3b9ec347ebf41360d6ea))
* **deps:** update module github.com/openai/openai-go/v3 to v3.23.0 ([#810](https://github.com/mogenius/mogenius-operator/issues/810)) ([95860a9](https://github.com/mogenius/mogenius-operator/commit/95860a992510b7f622b0765cd4876ad59d973165))
* **deps:** update module github.com/openai/openai-go/v3 to v3.24.0 ([#812](https://github.com/mogenius/mogenius-operator/issues/812)) ([9110c71](https://github.com/mogenius/mogenius-operator/commit/9110c710a11b32556264ec3f4e5c809a61d2b67b))
* **deps:** update module github.com/openai/openai-go/v3 to v3.26.0 ([1c9071a](https://github.com/mogenius/mogenius-operator/commit/1c9071a4efcb9e8c5320bf78969756695a9fa901))
* **deps:** update module github.com/shirou/gopsutil/v4 to v4.26.1 ([#768](https://github.com/mogenius/mogenius-operator/issues/768)) ([1b0a4c5](https://github.com/mogenius/mogenius-operator/commit/1b0a4c5a2690b9ac68cab3c1dda5bc0b929786c2))
* **deps:** update module github.com/valkey-io/valkey-go to v1.0.71 ([#765](https://github.com/mogenius/mogenius-operator/issues/765)) ([378b383](https://github.com/mogenius/mogenius-operator/commit/378b383cd4c4c91ee906b634d482c5d8334f889a))
* **deps:** update module github.com/valkey-io/valkey-go to v1.0.72 ([#796](https://github.com/mogenius/mogenius-operator/issues/796)) ([8831c1d](https://github.com/mogenius/mogenius-operator/commit/8831c1d3367fca34b8ea5f01df1b397dc4ae003e))
* **deps:** update module helm.sh/helm/v4 to v4.1.1 ([#784](https://github.com/mogenius/mogenius-operator/issues/784)) ([b63aa79](https://github.com/mogenius/mogenius-operator/commit/b63aa79db0d3284481aa0673041763c75eb1e8be))
* **deps:** update module sigs.k8s.io/controller-runtime to v0.23.1 ([#761](https://github.com/mogenius/mogenius-operator/issues/761)) ([5c2ff29](https://github.com/mogenius/mogenius-operator/commit/5c2ff29292f72563371ef3209d2c9fa98031f322))
* **deps:** update module sigs.k8s.io/controller-runtime to v0.23.3 ([dfc2cfe](https://github.com/mogenius/mogenius-operator/commit/dfc2cfe4320c3d24e9d45472faced5241c0afe5a))
* Dockerfile amd64 dynamic install for linux headers ([688da59](https://github.com/mogenius/mogenius-operator/commit/688da59c67b8ea4164d8f8aa63dc173e8b722203))
* error msg for tokenlimit in ui improved ([43a0ead](https://github.com/mogenius/mogenius-operator/commit/43a0eadf01af1c358ad0fc3fafa0e5c4410624a0))
* errors fixed ([0fd6307](https://github.com/mogenius/mogenius-operator/commit/0fd63076cd8e1cb09dd5e7f34aa94ca1729998c2))
* excessive token usage ([efadb8a](https://github.com/mogenius/mogenius-operator/commit/efadb8a92c4ad9bcf48aa49c0e8b9242507c2f54))
* filter ai tools for workspace viewer ([d1f4306](https://github.com/mogenius/mogenius-operator/commit/d1f430622d01b75cc9ca292597b64d1a64f70102))
* finalized go 1.26 usage ([47f715b](https://github.com/mogenius/mogenius-operator/commit/47f715b3dddc0695614736ababf8d1e189796c48))
* helm add fix ([bfd2ea6](https://github.com/mogenius/mogenius-operator/commit/bfd2ea66924efdaa2b32396d72540ee0b7653b33))
* helm chart name in dev ([55cf841](https://github.com/mogenius/mogenius-operator/commit/55cf841beaa97e1f0dc661bfe6648baba2f7d69d))
* helm v4 rollback fixed. update/install default repos. ([84cadf0](https://github.com/mogenius/mogenius-operator/commit/84cadf0a53b9d80f6b80e409d5671bbe064b1cd1))
* image tag name ([1f62e88](https://github.com/mogenius/mogenius-operator/commit/1f62e884a4653b1f639a2031df55a45abcad8b7d))
* implement first ai chat via WebSocket connection ([d304681](https://github.com/mogenius/mogenius-operator/commit/d3046816b6985afba9c1fbd55128ad6ff693e4bc))
* improved a loop ([444b88b](https://github.com/mogenius/mogenius-operator/commit/444b88b15d9413f33125491a0e57e1bf79c985ce))
* improved message handling. improved compression. ([276358e](https://github.com/mogenius/mogenius-operator/commit/276358e2b7bef5bce7d2f4f289c2c4d268d13477))
* improved pre-allocations for speedup. ([ffc4647](https://github.com/mogenius/mogenius-operator/commit/ffc4647127a2326b60ad82c7a44e1f675cd88e42))
* improved response filtering and jsonpath-filtering. ([df59a9e](https://github.com/mogenius/mogenius-operator/commit/df59a9e038b9482e95997dfcf9447f90b166f82f))
* json performance improvements because after running own benchmarks the lib switch gains 1.5-2.3x performance. ([63cbbe3](https://github.com/mogenius/mogenius-operator/commit/63cbbe39d112b1b0382bce16bef18697e1ed95e7))
* leaderelector improved. ([dc3c73e](https://github.com/mogenius/mogenius-operator/commit/dc3c73eb2d88364bfc24623808c4b9b27b1174fe))
* list all loadbalancer ips to support gateway api and all ingress-controller ([d7ba551](https://github.com/mogenius/mogenius-operator/commit/d7ba5513ce9e79d5c3677e089a4b8e5710a94164))
* locking improved ([45db9d4](https://github.com/mogenius/mogenius-operator/commit/45db9d455f619c34b04c8e6e753f5a11022e4ab4))
* maxtoolcall count added. ([4dc0bd9](https://github.com/mogenius/mogenius-operator/commit/4dc0bd93ccaf2b60c29af3e9b2a2ab9c7f16b8e9))
* maxtoolcall count added. ([885741d](https://github.com/mogenius/mogenius-operator/commit/885741dfa1a0ad59ff447b3612c0fc94ffe5a86e))
* mean latest bug fixed by refactoring latest logic for ai. ([3674e7b](https://github.com/mogenius/mogenius-operator/commit/3674e7b98e1088e8c3778020be4d6ae57e8745f7))
* minor improvements ([31ca17b](https://github.com/mogenius/mogenius-operator/commit/31ca17b39fa868497f1b2258ab4e5e7082b6f994))
* missing star added. stupid error. ([f64e76b](https://github.com/mogenius/mogenius-operator/commit/f64e76b83af66ececfd073b76019591410561818))
* more distributed cpu usage instead of peak-cpu every sec ([513caf2](https://github.com/mogenius/mogenius-operator/commit/513caf264578843bd2a187e7ef0910b042cd3572))
* more performance improvements. ([0e15157](https://github.com/mogenius/mogenius-operator/commit/0e151572f8ed882c1043e0541d2765460d65cd87))
* more performance improvements. ([4253163](https://github.com/mogenius/mogenius-operator/commit/4253163b2217cc8c1c2dc8fe0f5b6a35c4c64fd7))
* more performance improvements. ([f66353b](https://github.com/mogenius/mogenius-operator/commit/f66353b24c3d5492c84e6cd85dacecc88660b0a6))
* multi job client connection ([a1913b4](https://github.com/mogenius/mogenius-operator/commit/a1913b4d3f5113ae9a16243bbaf26bb0b55c945c))
* multiple improvements for speed gains. ([948b94b](https://github.com/mogenius/mogenius-operator/commit/948b94bd46f239114790f4a1d9cf0f78a19e1913))
* network traffic monitoring reactivated ([76d2959](https://github.com/mogenius/mogenius-operator/commit/76d2959338391a5cf8bb7f2f7d8c36afa26827c7))
* network traffic monitoring reactivated ([54c93b2](https://github.com/mogenius/mogenius-operator/commit/54c93b2f2954512bc6862b21e8df5891c1caa155))
* network traffic snoopy ([68efa47](https://github.com/mogenius/mogenius-operator/commit/68efa47daa7a3141d71b538db9943408696a40f3))
* operator crd gets/lists will now be using mirror-store to speed up things ([fb3f425](https://github.com/mogenius/mogenius-operator/commit/fb3f4253467d8f37d85d3856da808d3db107c77a))
* operator uses mirror-store in more locations to reduce k8s api calls ([06a1819](https://github.com/mogenius/mogenius-operator/commit/06a1819d41b8100ba96285c147efc50239452d71))
* performance improvements ([dce6888](https://github.com/mogenius/mogenius-operator/commit/dce68881d1e1f116641857255f9a463efdb5896a))
* performance improvements ([f7c8ae0](https://github.com/mogenius/mogenius-operator/commit/f7c8ae0de424cfc9e99b64bb264f80bd8be585f9))
* performance improvements ([e3bf5bf](https://github.com/mogenius/mogenius-operator/commit/e3bf5bf11d5cc14ac0a1b223bdedbb3dc91ab29e))
* performance improvements ([b9221a8](https://github.com/mogenius/mogenius-operator/commit/b9221a80ac4093b4a31ab20878382de9ab5b16ee))
* processing order for new ai events ([bedcd97](https://github.com/mogenius/mogenius-operator/commit/bedcd97e6562211aa9b55211fa65582dbab9e3de))
* progress. ([3ea787a](https://github.com/mogenius/mogenius-operator/commit/3ea787a05865b57bf1fc44603544740734a6ab0f))
* prometheus charts fix ([c3a4a3e](https://github.com/mogenius/mogenius-operator/commit/c3a4a3ed81bcd2e35a01501bbd9b0390efaaf09d))
* prompt and filters can now be exchanged during runtime. ([195bb76](https://github.com/mogenius/mogenius-operator/commit/195bb764ebe2b5001d521f4f65fd36c136db5546))
* rdb valkey problem ([673cbcf](https://github.com/mogenius/mogenius-operator/commit/673cbcfe5ac6461c61d6c0598600d5062b1ccc69))
* readme adjusted ([605939c](https://github.com/mogenius/mogenius-operator/commit/605939c9fe33bf54dfae453faa533ab94ab4f861))
* reconciler now also observes ai-filter-config. ([10c6089](https://github.com/mogenius/mogenius-operator/commit/10c60890ac4121feab2676bdcab0839b9baff2ab))
* reconciler performance improved ([779cea8](https://github.com/mogenius/mogenius-operator/commit/779cea8590e4297fbdefa7de713e6ef3fc1cf76e))
* refactoring ai tools, add helm tools ([daadc25](https://github.com/mogenius/mogenius-operator/commit/daadc2553d26b795b3e8fac848edc22211d900b1))
* refactoring injection ai prompts ([98e9703](https://github.com/mogenius/mogenius-operator/commit/98e9703fc5e2851d90f9be69f8cd70301fe1d048))
* release bot permissions ([060361a](https://github.com/mogenius/mogenius-operator/commit/060361a0662cd7a7ea0fe061a943109577aac390))
* remove env comment ([6fd4489](https://github.com/mogenius/mogenius-operator/commit/6fd448993c02075044ed770ad70bd878d23a6eb9))
* remove temperature setting from openai sdk because leading to problems ([6ad9a00](https://github.com/mogenius/mogenius-operator/commit/6ad9a0087627c5e8842ea0143f12a7ec9f9f6285))
* removed complexity to improve cpu usage ([72836de](https://github.com/mogenius/mogenius-operator/commit/72836de1abedeaa5207c372ef0cb0ddeddd776fa))
* removed complexity to improve cpu usage ([c8f4fed](https://github.com/mogenius/mogenius-operator/commit/c8f4fedc47294fdbf30eb387369b846a8ed3f8d0))
* removed fixed branch references ([e881d3c](https://github.com/mogenius/mogenius-operator/commit/e881d3c6c5348b1cf3489ef2971acf87d4079ff2))
* removed nfs mount to get rid of priviledged: true ([83575dd](https://github.com/mogenius/mogenius-operator/commit/83575dd881a9e44da205e2c8277b8093ec8a3f02))
* removed sleeps. ([0e74313](https://github.com/mogenius/mogenius-operator/commit/0e743136563515231af43b47f14737815037a7c9))
* removed systemcheck ([3967d6d](https://github.com/mogenius/mogenius-operator/commit/3967d6d9d36bdd3bb7379c46c8cdd1c82b344b13))
* removed usage/dependency of metrics-server ([8e8cf0e](https://github.com/mogenius/mogenius-operator/commit/8e8cf0e1fce4e4e055cd60b2418fe674949a7a2a))
* renaming. ([37776f3](https://github.com/mogenius/mogenius-operator/commit/37776f301401b7881e82d804a4c6d3862f022086))
* renovate operator installation fixed ([5db8f7f](https://github.com/mogenius/mogenius-operator/commit/5db8f7fe2a355ef97b5959caabdd21fae7302e9e))
* reset of orphan aitasks. ([5e6670c](https://github.com/mogenius/mogenius-operator/commit/5e6670c9040cac502f5e337a088b1b2763683ea8))
* routes for /stats and /socketapi in node-stats ([d173a08](https://github.com/mogenius/mogenius-operator/commit/d173a08fbf70600f7d2df6db6146bd593b020b3f))
* sdks updated. ([687b571](https://github.com/mogenius/mogenius-operator/commit/687b571db14c8881f1c02eb2199d313382736026))
* send COMPLETED message ([ad28c9d](https://github.com/mogenius/mogenius-operator/commit/ad28c9d9d94fa0d995fc0701932f9ee013d4f31e))
* service discovery for prometheus ([a6316b6](https://github.com/mogenius/mogenius-operator/commit/a6316b6e135e6d34fdc87426e187657792bbf262))
* switch back from arc runner to self-hosted ([9700342](https://github.com/mogenius/mogenius-operator/commit/970034241c2a23490c1b28239ccdd926243d77da))
* talos network traffic will now be discovered again ([ab5a7ca](https://github.com/mogenius/mogenius-operator/commit/ab5a7ca755424669df4aecab4d61bc40d014fc8c))
* talos network traffic will now be discovered again ([5b7323f](https://github.com/mogenius/mogenius-operator/commit/5b7323f898c702ba9cc5c6b4b498661654a0f18d))
* talos nodemetrics ([407e723](https://github.com/mogenius/mogenius-operator/commit/407e72302be1bec0759efefa2a37b7c9f3d02d4e))
* test arc runner arm64 ([38912c8](https://github.com/mogenius/mogenius-operator/commit/38912c86dd4a6cad1b15521837adaad6cd0f9ab4))
* toggle filter fix ([0ed7f20](https://github.com/mogenius/mogenius-operator/commit/0ed7f20febf2fa786a4f4eda3c3a0c82c12119cf))
* token usage reduced for anthropic ([4965c2f](https://github.com/mogenius/mogenius-operator/commit/4965c2f00790ad115633a9d66e0ad5a94c946a7f))
* token usage reduced for ollama and openai ([35b63fb](https://github.com/mogenius/mogenius-operator/commit/35b63fb160602b3fbf75e094fca1eac05a9873f0))
* toll naming changed ([7fce030](https://github.com/mogenius/mogenius-operator/commit/7fce03073dfbff236c6bd8f3028257bb76fb643f))
* toolcall-error fixed ([072b524](https://github.com/mogenius/mogenius-operator/commit/072b524e923567ec2032e9c74d8ea44213a9870c))
* traffic measurement bug fixed ([5b40429](https://github.com/mogenius/mogenius-operator/commit/5b40429e31c74d5969c79d30839216fb714c8a8f))
* trigger build ([23027d9](https://github.com/mogenius/mogenius-operator/commit/23027d9ca2b7c565dca3e612635386a26467ab8b))
* trigger build for build with snoopy v0.3.11 ([4fcf867](https://github.com/mogenius/mogenius-operator/commit/4fcf8674162f22016cd5ee68e992795659846542))
* unify tool definition and callability ([99462f8](https://github.com/mogenius/mogenius-operator/commit/99462f8923da26ff851f538ce3cf24eadc7be17d))
* unwanted automatic filter reset ([84974d4](https://github.com/mogenius/mogenius-operator/commit/84974d405e3711475fdfe1bbdfc1b37884fa522b))
* update ai filter configmap ([105d0e2](https://github.com/mogenius/mogenius-operator/commit/105d0e218ad4522b80f9419fbd1edc517783ae7d))
* updated snoopy and removed network device polling rate ([4ae57b4](https://github.com/mogenius/mogenius-operator/commit/4ae57b42fcfd98348dcfc9afe5f6d3f49659e3db))
* wrong dockfile name ([93914d1](https://github.com/mogenius/mogenius-operator/commit/93914d150b615c58f034a1986bc8e789982316bd))


### Features

* adding ingress for node-stats ([7bcb415](https://github.com/mogenius/mogenius-operator/commit/7bcb4159b24051ccbbbc08a277d012573d426a20))
* allow openai to get resources within cluster ([9868986](https://github.com/mogenius/mogenius-operator/commit/986898647e033c4bed45810342e7ff6b523e8a27))

# [2.19.0-develop.115](https://github.com/mogenius/mogenius-operator/compare/v2.19.0-develop.114...v2.19.0-develop.115) (2026-03-06)


### Bug Fixes

* minor improvements ([31ca17b](https://github.com/mogenius/mogenius-operator/commit/31ca17b39fa868497f1b2258ab4e5e7082b6f994))
* multi job client connection ([a1913b4](https://github.com/mogenius/mogenius-operator/commit/a1913b4d3f5113ae9a16243bbaf26bb0b55c945c))
* readme adjusted ([605939c](https://github.com/mogenius/mogenius-operator/commit/605939c9fe33bf54dfae453faa533ab94ab4f861))

# [2.19.0-develop.114](https://github.com/mogenius/mogenius-operator/compare/v2.19.0-develop.113...v2.19.0-develop.114) (2026-03-06)


### Bug Fixes

* removed fixed branch references ([e881d3c](https://github.com/mogenius/mogenius-operator/commit/e881d3c6c5348b1cf3489ef2971acf87d4079ff2))

# [2.19.0-develop.113](https://github.com/mogenius/mogenius-operator/compare/v2.19.0-develop.112...v2.19.0-develop.113) (2026-03-06)


### Bug Fixes

* **deps:** update module github.com/ollama/ollama to v0.17.7 ([af2ba07](https://github.com/mogenius/mogenius-operator/commit/af2ba07ae69f47331bef61adb21f0c74067dabf1))

# [2.19.0-develop.112](https://github.com/mogenius/mogenius-operator/compare/v2.19.0-develop.111...v2.19.0-develop.112) (2026-03-06)


### Bug Fixes

* **deps:** update module github.com/openai/openai-go/v3 to v3.26.0 ([1c9071a](https://github.com/mogenius/mogenius-operator/commit/1c9071a4efcb9e8c5320bf78969756695a9fa901))
* **deps:** update module sigs.k8s.io/controller-runtime to v0.23.3 ([dfc2cfe](https://github.com/mogenius/mogenius-operator/commit/dfc2cfe4320c3d24e9d45472faced5241c0afe5a))

# [2.19.0-develop.111](https://github.com/mogenius/mogenius-operator/compare/v2.19.0-develop.110...v2.19.0-develop.111) (2026-03-06)


### Bug Fixes

* adds claude files ([1bed2dd](https://github.com/mogenius/mogenius-operator/commit/1bed2dd5e919279f10bf262a062f96165df44e29))

# [2.19.0-develop.110](https://github.com/mogenius/mogenius-operator/compare/v2.19.0-develop.109...v2.19.0-develop.110) (2026-03-04)


### Bug Fixes

* toggle filter fix ([0ed7f20](https://github.com/mogenius/mogenius-operator/commit/0ed7f20febf2fa786a4f4eda3c3a0c82c12119cf))

# [2.19.0-develop.109](https://github.com/mogenius/mogenius-operator/compare/v2.19.0-develop.108...v2.19.0-develop.109) (2026-03-04)


### Bug Fixes

* add new pattern get/workload/pod-logs and get/workload/pod-events ([e77c2f0](https://github.com/mogenius/mogenius-operator/commit/e77c2f04d824ccafc101d161d9c10f4745380620))

# [2.19.0-develop.108](https://github.com/mogenius/mogenius-operator/compare/v2.19.0-develop.107...v2.19.0-develop.108) (2026-03-04)


### Bug Fixes

* **deps:** update module github.com/ollama/ollama to v0.17.6 ([#830](https://github.com/mogenius/mogenius-operator/issues/830)) ([e7a05bb](https://github.com/mogenius/mogenius-operator/commit/e7a05bbca8f298b306828a8bdf8faa28fdbee770))

# [2.19.0-develop.107](https://github.com/mogenius/mogenius-operator/compare/v2.19.0-develop.106...v2.19.0-develop.107) (2026-03-02)


### Bug Fixes

* service discovery for prometheus ([a6316b6](https://github.com/mogenius/mogenius-operator/commit/a6316b6e135e6d34fdc87426e187657792bbf262))

# [2.19.0-develop.106](https://github.com/mogenius/mogenius-operator/compare/v2.19.0-develop.105...v2.19.0-develop.106) (2026-03-01)


### Bug Fixes

* **deps:** update module github.com/ollama/ollama to v0.17.5 ([#829](https://github.com/mogenius/mogenius-operator/issues/829)) ([2412d81](https://github.com/mogenius/mogenius-operator/commit/2412d81fd55b42987263119bbdc9a826cc9dd13d))

# [2.19.0-develop.105](https://github.com/mogenius/mogenius-operator/compare/v2.19.0-develop.104...v2.19.0-develop.105) (2026-02-28)


### Bug Fixes

* **deps:** update module github.com/modelcontextprotocol/go-sdk to v1.4.0 ([#828](https://github.com/mogenius/mogenius-operator/issues/828)) ([5881fc5](https://github.com/mogenius/mogenius-operator/commit/5881fc574b0d1e222b98957b91fdfbf1009701fe))

# [2.19.0-develop.104](https://github.com/mogenius/mogenius-operator/compare/v2.19.0-develop.103...v2.19.0-develop.104) (2026-02-27)


### Bug Fixes

* **deps:** update kubernetes packages to v0.35.2 ([#827](https://github.com/mogenius/mogenius-operator/issues/827)) ([544109c](https://github.com/mogenius/mogenius-operator/commit/544109cbc1822a455770b48903776d3f72bbf747))

# [2.19.0-develop.103](https://github.com/mogenius/mogenius-operator/compare/v2.19.0-develop.102...v2.19.0-develop.103) (2026-02-27)


### Bug Fixes

* errors fixed ([0fd6307](https://github.com/mogenius/mogenius-operator/commit/0fd63076cd8e1cb09dd5e7f34aa94ca1729998c2))

# [2.19.0-develop.102](https://github.com/mogenius/mogenius-operator/compare/v2.19.0-develop.101...v2.19.0-develop.102) (2026-02-27)


### Bug Fixes

* network traffic snoopy ([68efa47](https://github.com/mogenius/mogenius-operator/commit/68efa47daa7a3141d71b538db9943408696a40f3))

# [2.19.0-develop.101](https://github.com/mogenius/mogenius-operator/compare/v2.19.0-develop.100...v2.19.0-develop.101) (2026-02-27)


### Bug Fixes

* **deps:** update module github.com/ollama/ollama to v0.17.4 ([#826](https://github.com/mogenius/mogenius-operator/issues/826)) ([e7e4ff7](https://github.com/mogenius/mogenius-operator/commit/e7e4ff7a0c2baac0b6e0ad44b235f1eae91aeedc))

# [2.19.0-develop.100](https://github.com/mogenius/mogenius-operator/compare/v2.19.0-develop.99...v2.19.0-develop.100) (2026-02-26)


### Bug Fixes

* bump snoopy version ([ac92d47](https://github.com/mogenius/mogenius-operator/commit/ac92d478b11cbae39ce2a61b34ef90078a5fa2d6))

# [2.19.0-develop.99](https://github.com/mogenius/mogenius-operator/compare/v2.19.0-develop.98...v2.19.0-develop.99) (2026-02-26)


### Bug Fixes

* trigger build for build with snoopy v0.3.11 ([4fcf867](https://github.com/mogenius/mogenius-operator/commit/4fcf8674162f22016cd5ee68e992795659846542))

# [2.19.0-develop.98](https://github.com/mogenius/mogenius-operator/compare/v2.19.0-develop.97...v2.19.0-develop.98) (2026-02-26)


### Bug Fixes

* network traffic monitoring reactivated ([76d2959](https://github.com/mogenius/mogenius-operator/commit/76d2959338391a5cf8bb7f2f7d8c36afa26827c7))

# [2.19.0-develop.97](https://github.com/mogenius/mogenius-operator/compare/v2.19.0-develop.96...v2.19.0-develop.97) (2026-02-26)


### Bug Fixes

* network traffic monitoring reactivated ([54c93b2](https://github.com/mogenius/mogenius-operator/commit/54c93b2f2954512bc6862b21e8df5891c1caa155))

# [2.19.0-develop.96](https://github.com/mogenius/mogenius-operator/compare/v2.19.0-develop.95...v2.19.0-develop.96) (2026-02-26)


### Bug Fixes

* image tag name ([1f62e88](https://github.com/mogenius/mogenius-operator/commit/1f62e884a4653b1f639a2031df55a45abcad8b7d))

# [2.19.0-develop.95](https://github.com/mogenius/mogenius-operator/compare/v2.19.0-develop.94...v2.19.0-develop.95) (2026-02-26)


### Bug Fixes

* talos network traffic will now be discovered again ([ab5a7ca](https://github.com/mogenius/mogenius-operator/commit/ab5a7ca755424669df4aecab4d61bc40d014fc8c))

# [2.19.0-develop.94](https://github.com/mogenius/mogenius-operator/compare/v2.19.0-develop.93...v2.19.0-develop.94) (2026-02-26)


### Bug Fixes

* talos network traffic will now be discovered again ([5b7323f](https://github.com/mogenius/mogenius-operator/commit/5b7323f898c702ba9cc5c6b4b498661654a0f18d))

# [2.19.0-develop.93](https://github.com/mogenius/mogenius-operator/compare/v2.19.0-develop.92...v2.19.0-develop.93) (2026-02-26)


### Bug Fixes

* helm chart name in dev ([55cf841](https://github.com/mogenius/mogenius-operator/commit/55cf841beaa97e1f0dc661bfe6648baba2f7d69d))

# [2.19.0-develop.92](https://github.com/mogenius/mogenius-operator/compare/v2.19.0-develop.91...v2.19.0-develop.92) (2026-02-26)


### Bug Fixes

* activate helm release in dev ([cea4269](https://github.com/mogenius/mogenius-operator/commit/cea4269a1c3abe3c5f1634543f0a3e79355dbc0b))

# [2.19.0-develop.91](https://github.com/mogenius/mogenius-operator/compare/v2.19.0-develop.90...v2.19.0-develop.91) (2026-02-26)


### Bug Fixes

* wrong dockfile name ([93914d1](https://github.com/mogenius/mogenius-operator/commit/93914d150b615c58f034a1986bc8e789982316bd))

# [2.19.0-develop.90](https://github.com/mogenius/mogenius-operator/compare/v2.19.0-develop.89...v2.19.0-develop.90) (2026-02-26)


### Bug Fixes

* add new Dockerfile ([49f1c45](https://github.com/mogenius/mogenius-operator/commit/49f1c45beda4d330c434dee7ea9e8cf374f1122b))
* add new github workflow actions ([1896616](https://github.com/mogenius/mogenius-operator/commit/18966165bf6767b70db4fd702a0e4dfcb76dadc1))
* add permissions: inherit to prepare ([e85f7ee](https://github.com/mogenius/mogenius-operator/commit/e85f7ee41cc98b88d57b669d8c98590587ff4a74))
* add release Token ([984a645](https://github.com/mogenius/mogenius-operator/commit/984a645412c27c2697ca83d47ff36b39160565af))
* add secrets inherit ([4a037c6](https://github.com/mogenius/mogenius-operator/commit/4a037c6e5e35dcead6c775afe05654c9aec918b7))
* change ref: main for SemVer ([eee94b4](https://github.com/mogenius/mogenius-operator/commit/eee94b4abc4bf7b7294bbc943d6776ad1e2f7aae))
* remove env comment ([6fd4489](https://github.com/mogenius/mogenius-operator/commit/6fd448993c02075044ed770ad70bd878d23a6eb9))

# [2.19.0-develop.89](https://github.com/mogenius/mogenius-operator/compare/v2.19.0-develop.88...v2.19.0-develop.89) (2026-02-26)


### Bug Fixes

* removed nfs mount to get rid of priviledged: true ([83575dd](https://github.com/mogenius/mogenius-operator/commit/83575dd881a9e44da205e2c8277b8093ec8a3f02))

# [2.19.0-develop.88](https://github.com/mogenius/mogenius-operator/compare/v2.19.0-develop.87...v2.19.0-develop.88) (2026-02-26)


### Bug Fixes

* helm add fix ([bfd2ea6](https://github.com/mogenius/mogenius-operator/commit/bfd2ea66924efdaa2b32396d72540ee0b7653b33))
* talos nodemetrics ([407e723](https://github.com/mogenius/mogenius-operator/commit/407e72302be1bec0759efefa2a37b7c9f3d02d4e))

# [2.19.0-develop.87](https://github.com/mogenius/mogenius-operator/compare/v2.19.0-develop.86...v2.19.0-develop.87) (2026-02-26)


### Bug Fixes

* prometheus charts fix ([c3a4a3e](https://github.com/mogenius/mogenius-operator/commit/c3a4a3ed81bcd2e35a01501bbd9b0390efaaf09d))

# [2.19.0-develop.86](https://github.com/mogenius/mogenius-operator/compare/v2.19.0-develop.85...v2.19.0-develop.86) (2026-02-26)


### Bug Fixes

* **deps:** update module github.com/go-git/go-git/v5 to v5.17.0 ([#814](https://github.com/mogenius/mogenius-operator/issues/814)) ([ba2dbdd](https://github.com/mogenius/mogenius-operator/commit/ba2dbddd02ede76d6298d3d27ef37583671ef7e2))

# [2.19.0-develop.85](https://github.com/mogenius/mogenius-operator/compare/v2.19.0-develop.84...v2.19.0-develop.85) (2026-02-25)


### Bug Fixes

* **deps:** update module github.com/bitnami-labs/sealed-secrets to v0.36.0 ([#813](https://github.com/mogenius/mogenius-operator/issues/813)) ([efce86d](https://github.com/mogenius/mogenius-operator/commit/efce86d6f685aa5ffb61b217538c54b57bed7eb6))

# [2.19.0-develop.84](https://github.com/mogenius/mogenius-operator/compare/v2.19.0-develop.83...v2.19.0-develop.84) (2026-02-25)


### Bug Fixes

* add new ai tools pod/event logs ([1650421](https://github.com/mogenius/mogenius-operator/commit/1650421e9e2242f1134d74f723495b8798bdc00e))

# [2.19.0-develop.83](https://github.com/mogenius/mogenius-operator/compare/v2.19.0-develop.82...v2.19.0-develop.83) (2026-02-25)


### Bug Fixes

* removed systemcheck ([3967d6d](https://github.com/mogenius/mogenius-operator/commit/3967d6d9d36bdd3bb7379c46c8cdd1c82b344b13))

# [2.19.0-develop.82](https://github.com/mogenius/mogenius-operator/compare/v2.19.0-develop.81...v2.19.0-develop.82) (2026-02-25)


### Bug Fixes

* removed usage/dependency of metrics-server ([8e8cf0e](https://github.com/mogenius/mogenius-operator/commit/8e8cf0e1fce4e4e055cd60b2418fe674949a7a2a))

# [2.19.0-develop.81](https://github.com/mogenius/mogenius-operator/compare/v2.19.0-develop.80...v2.19.0-develop.81) (2026-02-25)


### Bug Fixes

* **deps:** update module github.com/openai/openai-go/v3 to v3.24.0 ([#812](https://github.com/mogenius/mogenius-operator/issues/812)) ([9110c71](https://github.com/mogenius/mogenius-operator/commit/9110c710a11b32556264ec3f4e5c809a61d2b67b))

# [2.19.0-develop.80](https://github.com/mogenius/mogenius-operator/compare/v2.19.0-develop.79...v2.19.0-develop.80) (2026-02-24)


### Bug Fixes

* **deps:** update module github.com/cert-manager/cert-manager to v1.19.4 ([#811](https://github.com/mogenius/mogenius-operator/issues/811)) ([4d3c32d](https://github.com/mogenius/mogenius-operator/commit/4d3c32d026f6111a13c594fb7a588ea6165e6c05))

# [2.19.0-develop.79](https://github.com/mogenius/mogenius-operator/compare/v2.19.0-develop.78...v2.19.0-develop.79) (2026-02-24)


### Bug Fixes

* toolcall-error fixed ([072b524](https://github.com/mogenius/mogenius-operator/commit/072b524e923567ec2032e9c74d8ea44213a9870c))

# [2.19.0-develop.78](https://github.com/mogenius/mogenius-operator/compare/v2.19.0-develop.77...v2.19.0-develop.78) (2026-02-24)


### Bug Fixes

* token usage reduced for anthropic ([4965c2f](https://github.com/mogenius/mogenius-operator/commit/4965c2f00790ad115633a9d66e0ad5a94c946a7f))
* token usage reduced for ollama and openai ([35b63fb](https://github.com/mogenius/mogenius-operator/commit/35b63fb160602b3fbf75e094fca1eac05a9873f0))

# [2.19.0-develop.77](https://github.com/mogenius/mogenius-operator/compare/v2.19.0-develop.76...v2.19.0-develop.77) (2026-02-24)


### Bug Fixes

* **deps:** update module github.com/openai/openai-go/v3 to v3.23.0 ([#810](https://github.com/mogenius/mogenius-operator/issues/810)) ([95860a9](https://github.com/mogenius/mogenius-operator/commit/95860a992510b7f622b0765cd4876ad59d973165))

# [2.19.0-develop.76](https://github.com/mogenius/mogenius-operator/compare/v2.19.0-develop.75...v2.19.0-develop.76) (2026-02-23)


### Bug Fixes

* **deps:** update module github.com/ollama/ollama to v0.17.0 ([#809](https://github.com/mogenius/mogenius-operator/issues/809)) ([863ebcb](https://github.com/mogenius/mogenius-operator/commit/863ebcbda4e93939ec8a56c74f9ba28009e01086))

# [2.19.0-develop.75](https://github.com/mogenius/mogenius-operator/compare/v2.19.0-develop.74...v2.19.0-develop.75) (2026-02-23)


### Bug Fixes

* renovate operator installation fixed ([5db8f7f](https://github.com/mogenius/mogenius-operator/commit/5db8f7fe2a355ef97b5959caabdd21fae7302e9e))

# [2.19.0-develop.74](https://github.com/mogenius/mogenius-operator/compare/v2.19.0-develop.73...v2.19.0-develop.74) (2026-02-23)


### Bug Fixes

* error msg for tokenlimit in ui improved ([43a0ead](https://github.com/mogenius/mogenius-operator/commit/43a0eadf01af1c358ad0fc3fafa0e5c4410624a0))

# [2.19.0-develop.73](https://github.com/mogenius/mogenius-operator/compare/v2.19.0-develop.72...v2.19.0-develop.73) (2026-02-23)


### Bug Fixes

* unwanted automatic filter reset ([84974d4](https://github.com/mogenius/mogenius-operator/commit/84974d405e3711475fdfe1bbdfc1b37884fa522b))

# [2.19.0-develop.72](https://github.com/mogenius/mogenius-operator/compare/v2.19.0-develop.71...v2.19.0-develop.72) (2026-02-23)


### Bug Fixes

* excessive token usage ([efadb8a](https://github.com/mogenius/mogenius-operator/commit/efadb8a92c4ad9bcf48aa49c0e8b9242507c2f54))

# [2.19.0-develop.71](https://github.com/mogenius/mogenius-operator/compare/v2.19.0-develop.70...v2.19.0-develop.71) (2026-02-23)


### Bug Fixes

* ai chat tokens ([d9a651a](https://github.com/mogenius/mogenius-operator/commit/d9a651af339128ff1232719677e2b733eabf8cee))

# [2.19.0-develop.70](https://github.com/mogenius/mogenius-operator/compare/v2.19.0-develop.69...v2.19.0-develop.70) (2026-02-23)


### Bug Fixes

* finalized go 1.26 usage ([47f715b](https://github.com/mogenius/mogenius-operator/commit/47f715b3dddc0695614736ababf8d1e189796c48))

# [2.19.0-develop.69](https://github.com/mogenius/mogenius-operator/compare/v2.19.0-develop.68...v2.19.0-develop.69) (2026-02-23)


### Bug Fixes

* performance improvements ([dce6888](https://github.com/mogenius/mogenius-operator/commit/dce68881d1e1f116641857255f9a463efdb5896a))
* performance improvements ([f7c8ae0](https://github.com/mogenius/mogenius-operator/commit/f7c8ae0de424cfc9e99b64bb264f80bd8be585f9))
* performance improvements ([e3bf5bf](https://github.com/mogenius/mogenius-operator/commit/e3bf5bf11d5cc14ac0a1b223bdedbb3dc91ab29e))
* performance improvements ([b9221a8](https://github.com/mogenius/mogenius-operator/commit/b9221a80ac4093b4a31ab20878382de9ab5b16ee))
* traffic measurement bug fixed ([5b40429](https://github.com/mogenius/mogenius-operator/commit/5b40429e31c74d5969c79d30839216fb714c8a8f))

# [2.19.0-develop.68](https://github.com/mogenius/mogenius-operator/compare/v2.19.0-develop.67...v2.19.0-develop.68) (2026-02-23)


### Bug Fixes

* refactoring injection ai prompts ([98e9703](https://github.com/mogenius/mogenius-operator/commit/98e9703fc5e2851d90f9be69f8cd70301fe1d048))

# [2.19.0-develop.67](https://github.com/mogenius/mogenius-operator/compare/v2.19.0-develop.66...v2.19.0-develop.67) (2026-02-20)


### Bug Fixes

* rdb valkey problem ([673cbcf](https://github.com/mogenius/mogenius-operator/commit/673cbcfe5ac6461c61d6c0598600d5062b1ccc69))

# [2.19.0-develop.66](https://github.com/mogenius/mogenius-operator/compare/v2.19.0-develop.65...v2.19.0-develop.66) (2026-02-20)


### Bug Fixes

* updated snoopy and removed network device polling rate ([4ae57b4](https://github.com/mogenius/mogenius-operator/commit/4ae57b42fcfd98348dcfc9afe5f6d3f49659e3db))

# [2.19.0-develop.65](https://github.com/mogenius/mogenius-operator/compare/v2.19.0-develop.64...v2.19.0-develop.65) (2026-02-20)


### Bug Fixes

* improved a loop ([444b88b](https://github.com/mogenius/mogenius-operator/commit/444b88b15d9413f33125491a0e57e1bf79c985ce))
* more distributed cpu usage instead of peak-cpu every sec ([513caf2](https://github.com/mogenius/mogenius-operator/commit/513caf264578843bd2a187e7ef0910b042cd3572))
* removed complexity to improve cpu usage ([72836de](https://github.com/mogenius/mogenius-operator/commit/72836de1abedeaa5207c372ef0cb0ddeddd776fa))
* removed complexity to improve cpu usage ([c8f4fed](https://github.com/mogenius/mogenius-operator/commit/c8f4fedc47294fdbf30eb387369b846a8ed3f8d0))

# [2.19.0-develop.64](https://github.com/mogenius/mogenius-operator/compare/v2.19.0-develop.63...v2.19.0-develop.64) (2026-02-20)


### Bug Fixes

* missing star added. stupid error. ([f64e76b](https://github.com/mogenius/mogenius-operator/commit/f64e76b83af66ececfd073b76019591410561818))

# [2.19.0-develop.63](https://github.com/mogenius/mogenius-operator/compare/v2.19.0-develop.62...v2.19.0-develop.63) (2026-02-20)


### Bug Fixes

* **deps:** update module github.com/ollama/ollama to v0.16.3 ([#805](https://github.com/mogenius/mogenius-operator/issues/805)) ([9c1508b](https://github.com/mogenius/mogenius-operator/commit/9c1508befc0af361516bf7b8f87320b90aef9d12))

# [2.19.0-develop.62](https://github.com/mogenius/mogenius-operator/compare/v2.19.0-develop.61...v2.19.0-develop.62) (2026-02-20)


### Bug Fixes

* **deps:** update module github.com/anthropics/anthropic-sdk-go to v1.26.0 ([#804](https://github.com/mogenius/mogenius-operator/issues/804)) ([5d1c256](https://github.com/mogenius/mogenius-operator/commit/5d1c2560b8af7568ea3b233c498b2ce071f71b40))

# [2.19.0-develop.61](https://github.com/mogenius/mogenius-operator/compare/v2.19.0-develop.60...v2.19.0-develop.61) (2026-02-19)


### Bug Fixes

* chat token used reset after recording ([27fcb23](https://github.com/mogenius/mogenius-operator/commit/27fcb2300fe96691fc1d6392e2c398272392205a))

# [2.19.0-develop.60](https://github.com/mogenius/mogenius-operator/compare/v2.19.0-develop.59...v2.19.0-develop.60) (2026-02-19)


### Bug Fixes

* aI chat user role for workspace permission check ([7922566](https://github.com/mogenius/mogenius-operator/commit/79225662682f92adb49db51cd0f2ecd1f0c8abc0))

# [2.19.0-develop.59](https://github.com/mogenius/mogenius-operator/compare/v2.19.0-develop.58...v2.19.0-develop.59) (2026-02-19)


### Bug Fixes

* cache-improvements ([e7f391e](https://github.com/mogenius/mogenius-operator/commit/e7f391ef4eb6914aa9240a929510e9332bf169d1))
* locking improved ([45db9d4](https://github.com/mogenius/mogenius-operator/commit/45db9d455f619c34b04c8e6e753f5a11022e4ab4))
* operator crd gets/lists will now be using mirror-store to speed up things ([fb3f425](https://github.com/mogenius/mogenius-operator/commit/fb3f4253467d8f37d85d3856da808d3db107c77a))
* operator uses mirror-store in more locations to reduce k8s api calls ([06a1819](https://github.com/mogenius/mogenius-operator/commit/06a1819d41b8100ba96285c147efc50239452d71))
* reconciler performance improved ([779cea8](https://github.com/mogenius/mogenius-operator/commit/779cea8590e4297fbdefa7de713e6ef3fc1cf76e))

# [2.19.0-develop.58](https://github.com/mogenius/mogenius-operator/compare/v2.19.0-develop.57...v2.19.0-develop.58) (2026-02-19)


### Bug Fixes

* chat tokens used ([76fcb2b](https://github.com/mogenius/mogenius-operator/commit/76fcb2b6900b9b9f6c8993b5b50b574910ae5742))

# [2.19.0-develop.57](https://github.com/mogenius/mogenius-operator/compare/v2.19.0-develop.56...v2.19.0-develop.57) (2026-02-19)


### Bug Fixes

* processing order for new ai events ([bedcd97](https://github.com/mogenius/mogenius-operator/commit/bedcd97e6562211aa9b55211fa65582dbab9e3de))

# [2.19.0-develop.56](https://github.com/mogenius/mogenius-operator/compare/v2.19.0-develop.55...v2.19.0-develop.56) (2026-02-19)


### Bug Fixes

* remove temperature setting from openai sdk because leading to problems ([6ad9a00](https://github.com/mogenius/mogenius-operator/commit/6ad9a0087627c5e8842ea0143f12a7ec9f9f6285))

# [2.19.0-develop.55](https://github.com/mogenius/mogenius-operator/compare/v2.19.0-develop.54...v2.19.0-develop.55) (2026-02-19)


### Bug Fixes

* **deps:** update module github.com/anthropics/anthropic-sdk-go to v1.25.0 ([#800](https://github.com/mogenius/mogenius-operator/issues/800)) ([08eb702](https://github.com/mogenius/mogenius-operator/commit/08eb702270e1574911858ca7000f88d3df80d5ce))

# [2.19.0-develop.54](https://github.com/mogenius/mogenius-operator/compare/v2.19.0-develop.53...v2.19.0-develop.54) (2026-02-18)


### Bug Fixes

* **deps:** update module github.com/modelcontextprotocol/go-sdk to v1.3.1 ([#799](https://github.com/mogenius/mogenius-operator/issues/799)) ([777ed7a](https://github.com/mogenius/mogenius-operator/commit/777ed7a0e05ef1214ac857ee0e7d01e28aa71d4d))

# [2.19.0-develop.53](https://github.com/mogenius/mogenius-operator/compare/v2.19.0-develop.52...v2.19.0-develop.53) (2026-02-18)


### Bug Fixes

* **deps:** update module github.com/anthropics/anthropic-sdk-go to v1.24.0 ([#798](https://github.com/mogenius/mogenius-operator/issues/798)) ([7241390](https://github.com/mogenius/mogenius-operator/commit/72413907c32bb6ab1a958dd90bf737f08ecc0944))

# [2.19.0-develop.52](https://github.com/mogenius/mogenius-operator/compare/v2.19.0-develop.51...v2.19.0-develop.52) (2026-02-18)


### Bug Fixes

* **deps:** update module github.com/anthropics/anthropic-sdk-go to v1.23.0 ([#797](https://github.com/mogenius/mogenius-operator/issues/797)) ([eee6235](https://github.com/mogenius/mogenius-operator/commit/eee6235f9f4f5f784e5dcf878e80188236cd1d17))

# [2.19.0-develop.51](https://github.com/mogenius/mogenius-operator/compare/v2.19.0-develop.50...v2.19.0-develop.51) (2026-02-17)


### Bug Fixes

* **deps:** update module github.com/valkey-io/valkey-go to v1.0.72 ([#796](https://github.com/mogenius/mogenius-operator/issues/796)) ([8831c1d](https://github.com/mogenius/mogenius-operator/commit/8831c1d3367fca34b8ea5f01df1b397dc4ae003e))

# [2.19.0-develop.50](https://github.com/mogenius/mogenius-operator/compare/v2.19.0-develop.49...v2.19.0-develop.50) (2026-02-17)


### Bug Fixes

* **deps:** update module github.com/ollama/ollama to v0.16.2 ([#795](https://github.com/mogenius/mogenius-operator/issues/795)) ([2fa29d4](https://github.com/mogenius/mogenius-operator/commit/2fa29d405f6ddee6d7bf09889f429a02a46e1442))

# [2.19.0-develop.49](https://github.com/mogenius/mogenius-operator/compare/v2.19.0-develop.48...v2.19.0-develop.49) (2026-02-14)


### Bug Fixes

* **deps:** update module github.com/openai/openai-go/v3 to v3.22.0 ([#794](https://github.com/mogenius/mogenius-operator/issues/794)) ([19fa14f](https://github.com/mogenius/mogenius-operator/commit/19fa14f06b0ff45d97ef3b9ec347ebf41360d6ea))

# [2.19.0-develop.48](https://github.com/mogenius/mogenius-operator/compare/v2.19.0-develop.47...v2.19.0-develop.48) (2026-02-13)


### Bug Fixes

* **deps:** update module github.com/ollama/ollama to v0.16.1 ([#793](https://github.com/mogenius/mogenius-operator/issues/793)) ([adfb764](https://github.com/mogenius/mogenius-operator/commit/adfb764f109ef92492f99303cdf04390cba3ded9))

# [2.19.0-develop.47](https://github.com/mogenius/mogenius-operator/compare/v2.19.0-develop.46...v2.19.0-develop.47) (2026-02-13)


### Bug Fixes

* ai chat prompt config ([d9e6cb6](https://github.com/mogenius/mogenius-operator/commit/d9e6cb625c3b4194a5335a33e3f9710dcfc11aea))

# [2.19.0-develop.46](https://github.com/mogenius/mogenius-operator/compare/v2.19.0-develop.45...v2.19.0-develop.46) (2026-02-13)


### Bug Fixes

* filter ai tools for workspace viewer ([d1f4306](https://github.com/mogenius/mogenius-operator/commit/d1f430622d01b75cc9ca292597b64d1a64f70102))

# [2.19.0-develop.45](https://github.com/mogenius/mogenius-operator/compare/v2.19.0-develop.44...v2.19.0-develop.45) (2026-02-13)


### Bug Fixes

* **deps:** update module github.com/modelcontextprotocol/go-sdk to v1.3.0 ([#792](https://github.com/mogenius/mogenius-operator/issues/792)) ([d7aa16c](https://github.com/mogenius/mogenius-operator/commit/d7aa16c48ef54a1e5d6148b64cb8ef53f0feb801))

# [2.19.0-develop.44](https://github.com/mogenius/mogenius-operator/compare/v2.19.0-develop.43...v2.19.0-develop.44) (2026-02-12)


### Bug Fixes

* **deps:** update module github.com/bitnami-labs/sealed-secrets to v0.35.0 ([#791](https://github.com/mogenius/mogenius-operator/issues/791)) ([eb3fad4](https://github.com/mogenius/mogenius-operator/commit/eb3fad41985d5c0415a6457cbcf642b70bae960a))

# [2.19.0-develop.43](https://github.com/mogenius/mogenius-operator/compare/v2.19.0-develop.42...v2.19.0-develop.43) (2026-02-12)


### Bug Fixes

* **deps:** update module github.com/anthropics/anthropic-sdk-go to v1.22.1 ([#789](https://github.com/mogenius/mogenius-operator/issues/789)) ([baeafcb](https://github.com/mogenius/mogenius-operator/commit/baeafcb5f625266417b84837455d78984ccc822b))

# [2.19.0-develop.42](https://github.com/mogenius/mogenius-operator/compare/v2.19.0-develop.41...v2.19.0-develop.42) (2026-02-12)


### Bug Fixes

* **deps:** update kubernetes packages to v0.35.1 ([#788](https://github.com/mogenius/mogenius-operator/issues/788)) ([e6d89a7](https://github.com/mogenius/mogenius-operator/commit/e6d89a77b821991a17d6e80d3c7672f5a7f6ad33))

# [2.19.0-develop.41](https://github.com/mogenius/mogenius-operator/compare/v2.19.0-develop.40...v2.19.0-develop.41) (2026-02-11)


### Bug Fixes

* **deps:** update k8s.io/utils digest to b8788ab ([#786](https://github.com/mogenius/mogenius-operator/issues/786)) ([2505584](https://github.com/mogenius/mogenius-operator/commit/250558415fc065dbaf1afa70f81a0857737d5d7f))

# [2.19.0-develop.40](https://github.com/mogenius/mogenius-operator/compare/v2.19.0-develop.39...v2.19.0-develop.40) (2026-02-11)


### Bug Fixes

* add mcp client; update ai chat system prompt ([24fbd5a](https://github.com/mogenius/mogenius-operator/commit/24fbd5ada5b4957913f345956c7b956c733f1617))
* ai chat implemented ollama; refactoring mcp server connections ([0e263c7](https://github.com/mogenius/mogenius-operator/commit/0e263c77f307a699056bbe3f46a862c085c1b9c7))
* ai chat return values ([c88da73](https://github.com/mogenius/mogenius-operator/commit/c88da7319d0a42d9305addd9504d8022f73e762c))
* ai send input/output/session tokens ([b11a216](https://github.com/mogenius/mogenius-operator/commit/b11a216a78530f3f886107f2e008dc12224ac8ff))
* refactoring ai tools, add helm tools ([daadc25](https://github.com/mogenius/mogenius-operator/commit/daadc2553d26b795b3e8fac848edc22211d900b1))
* send COMPLETED message ([ad28c9d](https://github.com/mogenius/mogenius-operator/commit/ad28c9d9d94fa0d995fc0701932f9ee013d4f31e))

# [2.19.0-develop.39](https://github.com/mogenius/mogenius-operator/compare/v2.19.0-develop.38...v2.19.0-develop.39) (2026-02-10)


### Bug Fixes

* **deps:** update module github.com/go-git/go-git/v5 to v5.16.5 ([#783](https://github.com/mogenius/mogenius-operator/issues/783)) ([1c97cad](https://github.com/mogenius/mogenius-operator/commit/1c97cad1455373a9b36ce0f6f8d9e0cde5699d49))

# [2.19.0-develop.38](https://github.com/mogenius/mogenius-operator/compare/v2.19.0-develop.37...v2.19.0-develop.38) (2026-02-10)


### Bug Fixes

* **deps:** update module helm.sh/helm/v4 to v4.1.1 ([#784](https://github.com/mogenius/mogenius-operator/issues/784)) ([b63aa79](https://github.com/mogenius/mogenius-operator/commit/b63aa79db0d3284481aa0673041763c75eb1e8be))

# [2.19.0-develop.37](https://github.com/mogenius/mogenius-operator/compare/v2.19.0-develop.36...v2.19.0-develop.37) (2026-02-09)


### Bug Fixes

* implement first ai chat via WebSocket connection ([d304681](https://github.com/mogenius/mogenius-operator/commit/d3046816b6985afba9c1fbd55128ad6ff693e4bc))

# [2.19.0-develop.36](https://github.com/mogenius/mogenius-operator/compare/v2.19.0-develop.35...v2.19.0-develop.36) (2026-02-09)


### Bug Fixes

* progress. ([3ea787a](https://github.com/mogenius/mogenius-operator/commit/3ea787a05865b57bf1fc44603544740734a6ab0f))

# [2.19.0-develop.35](https://github.com/mogenius/mogenius-operator/compare/v2.19.0-develop.34...v2.19.0-develop.35) (2026-02-07)


### Bug Fixes

* **deps:** update module github.com/anthropics/anthropic-sdk-go to v1.22.0 ([#781](https://github.com/mogenius/mogenius-operator/issues/781)) ([dc308b4](https://github.com/mogenius/mogenius-operator/commit/dc308b45cad57bac345303778e0d52636df09f13))

# [2.19.0-develop.34](https://github.com/mogenius/mogenius-operator/compare/v2.19.0-develop.33...v2.19.0-develop.34) (2026-02-07)


### Bug Fixes

* **deps:** update module github.com/alecthomas/kong to v1.14.0 ([#780](https://github.com/mogenius/mogenius-operator/issues/780)) ([5688349](https://github.com/mogenius/mogenius-operator/commit/56883493344473e0b1e0439efa5f52f09d50197a))

# [2.19.0-develop.33](https://github.com/mogenius/mogenius-operator/compare/v2.19.0-develop.32...v2.19.0-develop.33) (2026-02-06)


### Bug Fixes

* **deps:** update module github.com/anthropics/anthropic-sdk-go to v1.21.0 ([#777](https://github.com/mogenius/mogenius-operator/issues/777)) ([70523ce](https://github.com/mogenius/mogenius-operator/commit/70523ce5e1dda22c36e527ade9d6092f8a31836c))

# [2.19.0-develop.32](https://github.com/mogenius/mogenius-operator/compare/v2.19.0-develop.31...v2.19.0-develop.32) (2026-02-06)


### Bug Fixes

* **deps:** update module github.com/ollama/ollama to v0.15.5 ([#779](https://github.com/mogenius/mogenius-operator/issues/779)) ([4fa84c4](https://github.com/mogenius/mogenius-operator/commit/4fa84c4a566df28ff89be6271dc65380775ff66c))

# [2.19.0-develop.31](https://github.com/mogenius/mogenius-operator/compare/v2.19.0-develop.30...v2.19.0-develop.31) (2026-02-05)


### Bug Fixes

* **deps:** update module github.com/openai/openai-go/v3 to v3.18.0 ([#778](https://github.com/mogenius/mogenius-operator/issues/778)) ([a2a03b8](https://github.com/mogenius/mogenius-operator/commit/a2a03b8b86e44e984d7ac31523fad301f800d62c))

# [2.19.0-develop.30](https://github.com/mogenius/mogenius-operator/compare/v2.19.0-develop.29...v2.19.0-develop.30) (2026-02-05)


### Bug Fixes

* Dockerfile amd64 dynamic install for linux headers ([688da59](https://github.com/mogenius/mogenius-operator/commit/688da59c67b8ea4164d8f8aa63dc173e8b722203))

# [2.19.0-develop.29](https://github.com/mogenius/mogenius-operator/compare/v2.19.0-develop.28...v2.19.0-develop.29) (2026-02-05)


### Bug Fixes

* list all loadbalancer ips to support gateway api and all ingress-controller ([d7ba551](https://github.com/mogenius/mogenius-operator/commit/d7ba5513ce9e79d5c3677e089a4b8e5710a94164))

# [2.19.0-develop.28](https://github.com/mogenius/mogenius-operator/compare/v2.19.0-develop.27...v2.19.0-develop.28) (2026-02-04)


### Bug Fixes

* **deps:** update module github.com/cert-manager/cert-manager to v1.19.3 ([#773](https://github.com/mogenius/mogenius-operator/issues/773)) ([f906d76](https://github.com/mogenius/mogenius-operator/commit/f906d760b8f359e4938fdcd7316e5cc3b91f8082))

# [2.19.0-develop.27](https://github.com/mogenius/mogenius-operator/compare/v2.19.0-develop.26...v2.19.0-develop.27) (2026-02-04)


### Bug Fixes

* ai tools bug fixed. ([baef5a7](https://github.com/mogenius/mogenius-operator/commit/baef5a7c8dbcb15fa64b81b91e0c97e5326d85f6))

# [2.19.0-develop.26](https://github.com/mogenius/mogenius-operator/compare/v2.19.0-develop.25...v2.19.0-develop.26) (2026-02-04)


### Bug Fixes

* switch back from arc runner to self-hosted ([9700342](https://github.com/mogenius/mogenius-operator/commit/970034241c2a23490c1b28239ccdd926243d77da))

# [2.19.0-develop.25](https://github.com/mogenius/mogenius-operator/compare/v2.19.0-develop.24...v2.19.0-develop.25) (2026-02-03)


### Bug Fixes

* update ai filter configmap ([105d0e2](https://github.com/mogenius/mogenius-operator/commit/105d0e218ad4522b80f9419fbd1edc517783ae7d))

# [2.19.0-develop.24](https://github.com/mogenius/mogenius-operator/compare/v2.19.0-develop.23...v2.19.0-develop.24) (2026-02-02)


### Bug Fixes

* helm v4 rollback fixed. update/install default repos. ([84cadf0](https://github.com/mogenius/mogenius-operator/commit/84cadf0a53b9d80f6b80e409d5671bbe064b1cd1))

# [2.19.0-develop.23](https://github.com/mogenius/mogenius-operator/compare/v2.19.0-develop.22...v2.19.0-develop.23) (2026-02-02)


### Bug Fixes

* test arc runner arm64 ([38912c8](https://github.com/mogenius/mogenius-operator/commit/38912c86dd4a6cad1b15521837adaad6cd0f9ab4))

# [2.19.0-develop.22](https://github.com/mogenius/mogenius-operator/compare/v2.19.0-develop.21...v2.19.0-develop.22) (2026-02-02)


### Bug Fixes

* add changelog to .releaserc ([90d68ed](https://github.com/mogenius/mogenius-operator/commit/90d68ed985ac84332fe907e6b9745aee4a0ed1d5))
* release bot permissions ([060361a](https://github.com/mogenius/mogenius-operator/commit/060361a0662cd7a7ea0fe061a943109577aac390))
