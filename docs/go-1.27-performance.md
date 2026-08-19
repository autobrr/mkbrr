# Go 1.27 performance evaluation

`go.mod` targets Go 1.27. This evaluation compares Go 1.26.6 and Go 1.27.0 before the version bump.

## Test environment

The tests used Linux on an AMD Ryzen 7 8745HS with the `amd64` v1 target. The performance tests used fixed CPU affinity.

The hash comparison used eight CPUs and ten interleaved samples. Each timed sample processed 20 repeats of these workloads:

- One 32 MiB file with 1 MiB pieces.
- Four 16 MiB files with 1 MiB pieces.

The JSON comparison used one CPU and 12 interleaved samples. Each sample ran for 200 ms.

`benchstat` calculated the medians and significance values. The comparison used Go 1.27 with these opt-out controls:

- `GOEXPERIMENT=simd`
- `GOEXPERIMENT=nojsonv2`
- `GOEXPERIMENT=nosizespecializedmalloc`

Before the version bump, the root test suite passed with both toolchains:

```text
GOTOOLCHAIN=go1.26.6 go test -count=1 ./...  PASS
GOTOOLCHAIN=go1.27.0 go test -count=1 ./...  PASS
```

The GUI test command stopped before compilation. The local checkout lacks `frontend/dist` and some GUI modules are not cached.

## Torrent hashing and verification

The values are median time per operation. A `p` value of 0.05 or more means that the measured change is not significant.

| Workload | Go 1.26 | Go 1.27 | 1.27 change | `p` | SIMD change from 1.27 | `p` |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| Hash one file | 5.857 ms | 5.616 ms | -4.1% | 0.218 | +12.1% | 0.043 |
| Hash four files | 8.677 ms | 9.039 ms | +4.2% | 0.280 | +2.5% | 0.631 |
| Verify one file | 5.887 ms | 6.224 ms | +5.7% | 0.218 | -2.2% | 0.739 |
| Verify four files | 9.461 ms | 9.491 ms | +0.3% | 0.853 | +0.8% | 0.853 |
| Geometric mean | 7.294 ms | 7.400 ms | +1.45% | | +3.18% | |

Go 1.27 produced no significant change in the four standard workloads. The SIMD build produced no consistent speed increase.

The one-file SIMD result was 12.1% slower. The other three results did not show a significant change.

mkbrr uses `crypto/sha1` in [`torrent/hasher.go`](../torrent/hasher.go) and [`torrent/verify.go`](../torrent/verify.go). It does not call the experimental SIMD packages.

The Go 1.27 notes do not report SHA-1 acceleration. The portable and architecture-specific SIMD packages require `GOEXPERIMENT=simd`.

The architecture-specific API is unstable. [SIMD notes](https://go.dev/doc/go1.27#simd), [architecture-specific SIMD notes](https://go.dev/doc/go1.27#archsimd)

## JSON

The JSON benchmark marshaled and unmarshaled a representative [`preset.Options`](../internal/preset/preset.go) value.

| Operation | Go 1.26 | Go 1.27 | Change | Old JSON engine on 1.27 |
| --- | ---: | ---: | ---: | ---: |
| Marshal | 580.0 ns | 959.3 ns | +65.4% | 567.8 ns |
| Unmarshal | 2.935 us | 1.347 us | -54.1% | 2.962 us |

The new engine increased marshal allocations from two to three. It decreased unmarshal allocations from 20 to eight.

`GOEXPERIMENT=nojsonv2` restored the Go 1.26 performance and allocation values. This control identifies the new JSON engine as the cause.

The new allocator decreased marshal time by 2.15%. It did not cause a significant unmarshal change.

mkbrr does not import `encoding/json` directly. The benchmark represents a GUI boundary payload, not a measured Wails request.

The absolute marshal increase is 379 ns per operation. This cost does not justify a JSON opt-out for the current workload.

Go 1.27 uses the v2 engine for the existing `encoding/json` API. The API keeps v1 behavior, but exact error text can change.

Go continues to support the v1 API. [JSON notes](https://go.dev/doc/go1.27#encodingjsonv2)

## Runtime and binary size

Go 1.27 uses specialized routines for some allocations smaller than 80 bytes. The Go team expects an overall gain near 1% for allocation-heavy programs.

The JSON control measured a 0.71% geometric-mean gain from this feature. No code change is necessary. [Runtime notes](https://go.dev/doc/go1.27#runtime)

A static default build produced these file sizes:

| Build | Bytes | Change from Go 1.26 |
| --- | ---: | ---: |
| Go 1.26 | 21,315,697 | |
| Go 1.27 | 22,596,188 | +1,280,491 |
| Go 1.27 with SIMD | 22,580,054 | +1,264,357 |
| Go 1.27 without specialized allocation | 22,450,851 | +1,135,154 |

Disabling specialized allocation decreased this build by 145,337 bytes. Other toolchain changes caused most of the size increase.

## Other release changes

Go 1.27 improves `compress/flate` speed. mkbrr does not use the standard archive or compression packages in a direct path.

A compression benchmark does not represent a current mkbrr workload. [Compression notes](https://go.dev/doc/go1.27#compressflate)

The new `hash/maphash` API does not apply to torrent SHA-1 pieces. [Hash notes](https://go.dev/doc/go1.27#hashmaphash)

## Decision

The CLI tests support a normal Go 1.27 upgrade. Performance does not require an urgent upgrade.

Keep the current hashing code. Do not enable `GOEXPERIMENT=simd`, and do not replace `crypto/sha1` with custom SIMD code.

Keep the default JSON engine. Before a release, build the complete GUI and test its request and response paths.
