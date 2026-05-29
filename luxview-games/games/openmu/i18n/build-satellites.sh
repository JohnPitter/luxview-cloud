#!/usr/bin/env bash
# build-satellites.sh — compila as satellite assemblies pt-BR do OpenMU.
#
# Fonte da verdade: json/<Short>/<ResxBase>.json  (ex.: json/Web.Map/Resources.json)
#   <Short>    -> nome curto do assembly (assembly = MUnique.OpenMU.<Short>)
#   <ResxBase> -> nome base do recurso (.../Properties/<ResxBase>.resx no OpenMU)
#
# Para cada <Short>, gera um projeto stub resource-only com AssemblyName e
# versão casando a imagem oficial (0.9.9.0, sem strong name), converte cada
# JSON em Properties/<ResxBase>.pt-BR.resx (via make-resx.py) e compila a
# satellite pt-BR/MUnique.OpenMU.<Short>.resources.dll.
#
# Saída: ./out/pt-BR/*.resources.dll  (overlay direto em /opt/openmu/pt-BR).
#
# Requer: dotnet SDK 10 + python3.
set -euo pipefail

I18N_DIR="$(cd "$(dirname "$0")" && pwd)"
SRC="$I18N_DIR/json"
OUT="$I18N_DIR/out"
VERSION="${OPENMU_ASM_VERSION:-0.9.9.0}"

WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

rm -rf "$OUT"
mkdir -p "$OUT/pt-BR"

shopt -s nullglob
for asmdir in "$SRC"/*/; do
  short="$(basename "$asmdir")"
  asm="MUnique.OpenMU.$short"
  proj="$WORK/$short"
  mkdir -p "$proj/Properties"

  for json in "$asmdir"*.json; do
    base="$(basename "$json" .json)"
    python3 "$I18N_DIR/make-resx.py" "$json" "$proj/Properties/$base.pt-BR.resx"
  done

  cat > "$proj/stub.csproj" <<EOF
<Project Sdk="Microsoft.NET.Sdk">
  <PropertyGroup>
    <TargetFramework>net10.0</TargetFramework>
    <AssemblyName>$asm</AssemblyName>
    <RootNamespace>$asm</RootNamespace>
    <AssemblyVersion>$VERSION</AssemblyVersion>
    <FileVersion>$VERSION</FileVersion>
    <Version>$VERSION</Version>
    <Nullable>disable</Nullable>
    <GenerateDocumentationFile>false</GenerateDocumentationFile>
    <SatelliteResourceLanguages>pt-BR</SatelliteResourceLanguages>
  </PropertyGroup>
</Project>
EOF
  printf '// resource-only stub for satellite assembly generation\n' > "$proj/_Stub.cs"

  echo ">> compilando satellite de $asm ..."
  dotnet build "$proj/stub.csproj" -c Release -o "$proj/bin" >/dev/null
  cp "$proj/bin/pt-BR/$asm.resources.dll" "$OUT/pt-BR/"
done

echo ">> Satellites geradas em $OUT/pt-BR:"
ls -1 "$OUT/pt-BR"
