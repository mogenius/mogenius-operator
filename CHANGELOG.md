# Changelog

## [2.27.1](https://github.com/mogenius/mogenius-operator/compare/v2.27.0...v2.27.1) (2026-08-06)


### Bug Fixes

* **helm:** stop hardcoding GOMEMLIMIT, it caused a GC death spiral (MOG-4518) ([fc2bac3](https://github.com/mogenius/mogenius-operator/commit/fc2bac3bf97936b1a93335c0b15c39797f6096d4))
* merge develop to main for the last time ([93ec410](https://github.com/mogenius/mogenius-operator/commit/93ec410fde0837b350e0684728127dc414f11631))
* **reconciler:** bound the store-readiness wait instead of requeueing forever ([406f96e](https://github.com/mogenius/mogenius-operator/commit/406f96e276f2fa8bd9e2ef882826233f573d6ea8))


### Performance Improvements

* **store:** read resource lists through the index instead of scanning ([14c2473](https://github.com/mogenius/mogenius-operator/commit/14c24735fbb2a2aad83a4a1fcf5538f4c48790e1))
