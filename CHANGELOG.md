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
