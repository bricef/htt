# Third-party assets

The GUI's page uses these files as they were published, unchanged. They
are embedded in the htt binary (`go:embed`), so the page fetches nothing
from the network. Each was downloaded from jsDelivr's mirror of npm on
2026-11-19 and checked against the npm registry's tarball for its
version, whose sha512 integrity the registry publishes (ECharts also
against cdnjs). To update one, download the new version, check it the
same way, and update its line here.

## Apache ECharts 6.1.0

Apache-2.0: `echarts/LICENSE` and `echarts/NOTICE`. ECharts bundles
code under BSD 3-Clause from d3 (`echarts/LICENSE-d3`, which its LICENSE
names) and its renderer, ZRender 6.1.0 (`echarts/LICENSE-zrender`).

| File | Source | sha256 |
|---|---|---|
| `echarts/echarts.min.js` | https://cdn.jsdelivr.net/npm/echarts@6.1.0/dist/echarts.min.js | `b66b25aeb4df84e33199dc21694014d336d222cbd9deb0e5a7c14bd6aa0d0fd0` |
| `echarts/LICENSE` | https://cdn.jsdelivr.net/npm/echarts@6.1.0/LICENSE | `634293835b43a6dd2094fa39182a3d9a6b9ca43b7fdb9ac354e8037af2a3093a` |
| `echarts/NOTICE` | https://cdn.jsdelivr.net/npm/echarts@6.1.0/NOTICE | `d491d358344f842685c1b1585970999db65fe30ecf7ef3867af8814f4016c016` |
| `echarts/LICENSE-d3` | https://cdn.jsdelivr.net/npm/echarts@6.1.0/licenses/LICENSE-d3 | `e1211892da0b0e0585b7aebe8f98c1274fba15bafe47fa1f4ee8a7a502c06304` |
| `echarts/LICENSE-zrender` | https://cdn.jsdelivr.net/npm/zrender@6.1.0/LICENSE | `a681e9d183c72b4af4856935893805eb48c08f0f5a3f260203db7444c20fb98a` |

## IBM Plex Sans 2.0.0 and IBM Plex Mono 2.5.0

The npm packages `@ibm/plex-sans` and `@ibm/plex-mono`: static woff2,
their Latin1 subsets (Basic Latin, Latin-1 and common punctuation such
as dashes and quotes); Sans in 400, 500 and 600, Mono in 400 and 500.
SIL Open Font License 1.1: `plex/LICENSE.txt`, the same file in both
packages (same sha256).

| File | Source | sha256 |
|---|---|---|
| `plex/IBMPlexSans-Regular-Latin1.woff2` | https://cdn.jsdelivr.net/npm/@ibm/plex-sans@2.0.0/fonts/split/woff2/IBMPlexSans-Regular-Latin1.woff2 | `bdb53f2af84b15e0a742428f5e199dacc3e6fd60776f0aebd9f700e2c97c960f` |
| `plex/IBMPlexSans-Medium-Latin1.woff2` | https://cdn.jsdelivr.net/npm/@ibm/plex-sans@2.0.0/fonts/split/woff2/IBMPlexSans-Medium-Latin1.woff2 | `668a0d9eabbfac0925f431988f56222b8b989ad66a0a78e91bc5889f73fe274a` |
| `plex/IBMPlexSans-SemiBold-Latin1.woff2` | https://cdn.jsdelivr.net/npm/@ibm/plex-sans@2.0.0/fonts/split/woff2/IBMPlexSans-SemiBold-Latin1.woff2 | `8251b7fc61b580c2f52550fa69b3c49fb94fc333815ad301ca6b1f6d15420f85` |
| `plex/IBMPlexMono-Regular-Latin1.woff2` | https://cdn.jsdelivr.net/npm/@ibm/plex-mono@2.5.0/fonts/split/woff2/IBMPlexMono-Regular-Latin1.woff2 | `e8993d946649b9d01abb1ed06d574b19d8ea3e66b5c3948602db335c44c18e56` |
| `plex/IBMPlexMono-Medium-Latin1.woff2` | https://cdn.jsdelivr.net/npm/@ibm/plex-mono@2.5.0/fonts/split/woff2/IBMPlexMono-Medium-Latin1.woff2 | `41201b658a328b9d00368215c2f1102770f80b15952ab82631e4006255e6365d` |
| `plex/LICENSE.txt` | https://cdn.jsdelivr.net/npm/@ibm/plex-sans@2.0.0/LICENSE.txt (and `@ibm/plex-mono@2.5.0`) | `7e6b2818edbd8f6a01ae80641cc8f16a51080d08fb4e532be3a0b6f74adb07da` |
