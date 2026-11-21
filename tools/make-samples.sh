#!/bin/bash
# Rebuilds sound/notes/*.wav from the Salamander Grand Piano V3 samples
# (Alexander Holm, CC BY 3.0, https://github.com/sfzinstruments/SalamanderGrandPiano).
#
# Usage: tools/make-samples.sh <dir with the Salamander FLACs named C3.flac, Ds3.flac, Fs3.flac, A3.flac … C6.flac>
#
# Salamander samples every minor third (C, D#, F#, A). The pentatonic notes it
# lacks are pitched from the nearest one by at most one semitone, as a sampler
# would: D = D# − 1, E = D# + 1, G = F# + 1.
set -euo pipefail
src="${1:?usage: $0 <salamander dir>}"
out="$(cd "$(dirname "$0")/.." && pwd)/sound/notes"
mkdir -p "$out"

# note name : source file : semitones, the C major pentatonic C1 … C8
notes=()
for o in 1 2 3 4 5 6 7; do
  notes+=(C$o:C$o:0 D$o:Ds$o:-1 E$o:Ds$o:1 G$o:Fs$o:1 A$o:A$o:0)
done
notes+=(C8:C8:0)

for n in "${notes[@]}"; do
  IFS=: read -r name file semis <<<"$n"
  ratio=$(awk -v s="$semis" 'BEGIN { printf "%.6f", 2^(s/12) }')
  # Low notes ring longer than high ones; keeping each about as long as it
  # is heard keeps the app small and NSGo's decoded audio under its 16 MiB.
  case "$name" in
    *1|*2) len=2.2 ;;
    *6) len=1.5 ;;
    *7|*8) len=1.1 ;;
    *) len=1.8 ;;
  esac
  fade=$(awk -v l="$len" 'BEGIN { print l * 0.35 }')
  # Pitch by resampling, fold to mono, start at the attack, keep len seconds
  # with a fade (the Swift app released each note after 1.8 s).
  shape="asetrate=48000*$ratio,aresample=44100,pan=mono|c0=0.5*c0+0.5*c1,\
silenceremove=start_periods=1:start_threshold=-50dB,atrim=0:$len,\
afade=t=out:st=$(awk -v l="$len" -v f="$fade" 'BEGIN { print l - f }'):d=$fade"
  # Then raise or lower the whole note so its peak sits at −3 dBFS, keeping
  # the piano's own decay.
  peak=$(ffmpeg -v info -i "$src/$file.flac" -af "$shape,volumedetect" -f null - 2>&1 |
    sed -n 's/.*max_volume: \(-*[0-9.]*\) dB.*/\1/p')
  ffmpeg -v error -y -i "$src/$file.flac" -af "$shape,volume=$(awk -v p="$peak" 'BEGIN { print -3 - p }')dB" \
    -ar 44100 -ac 1 -c:a pcm_s16le "$out/$name.wav"
done
echo "wrote $(ls "$out" | wc -l | tr -d ' ') notes to $out"
