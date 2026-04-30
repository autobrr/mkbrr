#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TMP_ROOT="$(mktemp -d "${TMPDIR:-/tmp}/mkbrr-bench.XXXXXX")"
GO_BIN="${GO_BIN:-}"

if [ -z "$GO_BIN" ]; then
  if command -v go >/dev/null 2>&1; then
    GO_BIN="$(command -v go)"
  elif [ -x /usr/local/go/bin/go ]; then
    GO_BIN="/usr/local/go/bin/go"
  else
    echo "go binary not found; set GO_BIN=/path/to/go" >&2
    exit 1
  fi
fi

TIME_RUNNER=()
if [ -x /usr/bin/time ]; then
  if /usr/bin/time -lp true >/dev/null 2>&1; then
    TIME_RUNNER=(/usr/bin/time -lp)
  elif /usr/bin/time -v true >/dev/null 2>&1; then
    TIME_RUNNER=(/usr/bin/time -v)
  else
    TIME_RUNNER=(/usr/bin/time)
  fi
elif [ -x /bin/time ]; then
  if /bin/time -lp true >/dev/null 2>&1; then
    TIME_RUNNER=(/bin/time -lp)
  elif /bin/time -v true >/dev/null 2>&1; then
    TIME_RUNNER=(/bin/time -v)
  else
    TIME_RUNNER=(/bin/time)
  fi
fi

run_timed() {
  if [ "${#TIME_RUNNER[@]}" -gt 0 ]; then
    "${TIME_RUNNER[@]}" "$@"
    return
  fi

  TIMEFORMAT=$'real %3R\nuser %3U\nsys %3S'
  time "$@"
}

cleanup() {
  if command -v trash >/dev/null 2>&1; then
    trash "$TMP_ROOT" >/dev/null 2>&1 || rm -rf "$TMP_ROOT"
  else
    rm -rf "$TMP_ROOT"
  fi
}
trap cleanup EXIT

PROFILE="${PROFILE:-season}"
RUNS="${RUNS:-5}"
WARMUP="${WARMUP:-1}"
BASELINE_REF="${BASELINE_REF:-}"
SEASON_FILES="${SEASON_FILES:-8}"
SEASON_FILE_MIB="${SEASON_FILE_MIB:-256}"
SINGLE_FILE_MIB="${SINGLE_FILE_MIB:-2048}"
ALBUM_FILES="${ALBUM_FILES:-12}"
ALBUM_FILE_MIB="${ALBUM_FILE_MIB:-64}"

build_binary() {
  local source_dir="$1"
  local output_path="$2"
  (
    cd "$source_dir"
    "$GO_BIN" build -o "$output_path" .
  )
}

trash_path() {
  local path="$1"
  if [ ! -e "$path" ]; then
    return 0
  fi
  if command -v trash >/dev/null 2>&1; then
    trash "$path" >/dev/null 2>&1 || rm -rf "$path"
  else
    rm -rf "$path"
  fi
}

write_file_mib() {
  local path="$1"
  local size_mib="$2"
  dd if=/dev/zero of="$path" bs=1M count="$size_mib" status=none
}

prepare_fixture() {
  local fixture_dir="$TMP_ROOT/fixture"
  mkdir -p "$fixture_dir"

  case "$PROFILE" in
    season)
      local season_dir="$fixture_dir/season-pack"
      mkdir -p "$season_dir"
      local index
      for index in $(seq 1 "$SEASON_FILES"); do
        printf -v episode_path "%s/Episode.S01E%02d.mkv" "$season_dir" "$index"
        write_file_mib "$episode_path" "$SEASON_FILE_MIB"
      done
      printf '%s\n' "$season_dir"
      ;;
    single)
      local single_path="$fixture_dir/single-file.bin"
      write_file_mib "$single_path" "$SINGLE_FILE_MIB"
      printf '%s\n' "$single_path"
      ;;
    album)
      local album_dir="$fixture_dir/flac-album"
      mkdir -p "$album_dir"
      local track
      for track in $(seq 1 "$ALBUM_FILES"); do
        printf -v track_path "%s/%02d-track.flac" "$album_dir" "$track"
        write_file_mib "$track_path" "$ALBUM_FILE_MIB"
      done
      printf '%s\n' "$album_dir"
      ;;
    *)
      echo "unsupported PROFILE: $PROFILE" >&2
      exit 1
      ;;
  esac
}

CURRENT_BIN="$TMP_ROOT/mkbrr-current"
build_binary "$ROOT_DIR" "$CURRENT_BIN"

BASELINE_BIN=""
if [ -n "$BASELINE_REF" ]; then
  BASELINE_SRC="$TMP_ROOT/baseline-src"
  mkdir -p "$BASELINE_SRC"
  git -C "$ROOT_DIR" archive "$BASELINE_REF" | tar -x -C "$BASELINE_SRC"
  BASELINE_BIN="$TMP_ROOT/mkbrr-baseline"
  build_binary "$BASELINE_SRC" "$BASELINE_BIN"
fi

FIXTURE_PATH="$(prepare_fixture)"
CURRENT_OUT="$TMP_ROOT/current.torrent"
BASELINE_OUT="$TMP_ROOT/baseline.torrent"

echo "Profile: $PROFILE"
echo "Fixture: $FIXTURE_PATH"
echo "Runs: $RUNS"
if [ -n "$BASELINE_BIN" ]; then
  echo "Baseline ref: $BASELINE_REF"
fi

if command -v hyperfine >/dev/null 2>&1; then
  PREPARE_CMD="$(command -v bash) -lc '$(typeset -f trash_path); trash_path \"$CURRENT_OUT\"; trash_path \"$BASELINE_OUT\"'"
  COMMANDS=(
    "$CURRENT_BIN create \"$FIXTURE_PATH\" --quiet --output \"$CURRENT_OUT\""
  )
  if [ -n "$BASELINE_BIN" ]; then
    COMMANDS+=(
      "$BASELINE_BIN create \"$FIXTURE_PATH\" --quiet --output \"$BASELINE_OUT\""
    )
  fi

  hyperfine \
    --warmup "$WARMUP" \
    --runs "$RUNS" \
    --prepare "$PREPARE_CMD" \
    "${COMMANDS[@]}"
else
  if [ "${#TIME_RUNNER[@]}" -eq 0 ]; then
    echo "hyperfine not found; falling back to bash built-in time" >&2
  else
    echo "hyperfine not found; falling back to ${TIME_RUNNER[*]}" >&2
  fi
  current_run=1
  while [ "$current_run" -le "$RUNS" ]; do
    trash_path "$CURRENT_OUT"
    echo
    echo "current run $current_run/$RUNS"
    run_timed "$CURRENT_BIN" create "$FIXTURE_PATH" --quiet --output "$CURRENT_OUT"

    if [ -n "$BASELINE_BIN" ]; then
      trash_path "$BASELINE_OUT"
      echo
      echo "baseline run $current_run/$RUNS"
      run_timed "$BASELINE_BIN" create "$FIXTURE_PATH" --quiet --output "$BASELINE_OUT"
    fi

    current_run=$((current_run + 1))
  done
fi
