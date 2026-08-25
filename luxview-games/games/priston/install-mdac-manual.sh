#!/bin/bash
# Instalação manual do MDAC 2.8 SP1 em um prefixo win32 do Wine, seguindo o
# layout oficial dos .inf do pacote (sqloldb.inf/mdacxpak.inf). O instalador
# oficial (mdac_typ.exe) falha com status 43 no wine moderno.
# Requer: /opt/mdac/x extraído de MDAC_TYP.EXE e $WINEPREFIX inicializado.
set -uo pipefail

MDAC_SRC="${MDAC_SRC:-/opt/mdac/x}"
C="$WINEPREFIX/drive_c"
S32="$C/windows/system32"
OLEDB="$C/Program Files/Common Files/System/OLE DB"
ADO="$C/Program Files/Common Files/System/ADO"

log() { echo "[mdac] $*"; }

mkdir -p "$S32" "$OLEDB" "$ADO"

# 1) núcleo ODBC/netlibs em system32 (DestinationDirs=11)
cp -f "$MDAC_SRC"/sqloldb/msdart.dll "$S32/"
cp -f "$MDAC_SRC"/sqlnet/{dbnetlib.dll,dbmsgnet.dll,dbmsrpcn.dll,dbnmpntw.dll,cliconfg.dll,cliconfg.rll,sqlunirl.dll} "$S32/"
cp -f "$MDAC_SRC"/sqlodbc/{sqlsrv32.dll,sqlsrv32.rll,odbcbcp.dll} "$S32/"
cp -f "$MDAC_SRC"/mdacxpak/{ODBC32.dll,ODBCINT.dll,ODBCCP32.dll,ODBCCR32.dll,ODBCCU32.dll,ODBCTRAC.dll,ODBC32GT.dll,DS32GT.dll} "$S32/"

# 2) OLE DB providers/services em Common Files\System\OLE DB (DestinationDirs=OLEDB)
cp -f "$MDAC_SRC"/sqloldb/{sqloledb.dll,sqloledb.rll} "$OLEDB/"
cp -f "$MDAC_SRC"/mdacxpak/{oledb32.dll,oledb32a.dll,oledb32r.dll,msdaps.dll,msxactps.dll,msdadc.dll,msdaenum.dll,msdaer.dll,msdaurl.dll,msdatt.dll,msdasql.dll,msdasqlr.dll,msdasc.dll,msdatl3.dll,simpdata.tlb} "$OLEDB/"

# 3) ADO em Common Files\System\ADO
cp -f "$MDAC_SRC"/mdacxpak/{msado15.dll,msador15.dll,msader15.dll,msadrh15.dll,msADOX.dll,msjro.dll,msdatsrc.tlb,msado20.tlb,msado21.tlb,msado25.tlb,msado26.tlb,msado27.tlb} "$ADO/"

# 4) remover cópias antigas de system32 que não pertencem ao layout oficial
for f in sqloledb.dll sqloledb.rll oledb32.dll oledb32a.dll oledb32r.dll msdaps.dll msdaenum.dll msdaer.dll msdadc.dll msdatt.dll msdatl3.dll msxactps.dll msdasql.dll msdasqlr.dll msdasc.dll msado15.dll msador15.dll msader15.dll msadrh15.dll msdaosp.dll; do
  rm -f "$S32/$f"
done

# 5) overrides + DSN em UM ÚNICO import .reg — chamadas sequenciais de
#    "wine reg add" (uma por DLL) falham silenciosamente aqui sob carga;
#    regedit /S com um arquivo único é o padrão confiável (mesmo usado no
#    entrypoint.sh para o registro do jogo).
# ATENÇÃO: em arquivo .reg o caminho da CHAVE usa barra SIMPLES (barra dupla
# só dentro de valores string). Com barra dupla o regedit ignora o bloco e
# NENHUM override é aplicado — o Wine então usa seus builtins incompletos de
# msado15 (quebra o login do jogo) e odbc32 (quebra o clã).
REGFILE="$WINEPREFIX/drive_c/windows/temp/mdac-overrides.reg"
{
  echo 'REGEDIT4'
  echo ''
  echo '[HKEY_LOCAL_MACHINE\Software\Wine\DllOverrides]'
  for dll in sqloledb oledb32 oledb32a msdasc msdaps msdaenum msdaer msdadc msdatt msdatl3 msxactps msdasql msado15 msador15 msadox msjro odbc32 odbccp32 odbcint odbccr32 odbccu32 odbctrac sqlsrv32 dbnetlib dbmsgnet dbmsrpcn dbnmpntw cliconfg sqlunirl msdart; do
    printf '"%s"="native,builtin"\n' "$dll"
  done
} > "$REGFILE"
wine regedit /S "C:\\windows\\temp\\mdac-overrides.reg" || log "regedit overrides FALHOU rc=$?"
log "layout + overrides aplicados."

# odbc32 nativo (copiado do MDAC) colide em case-insensitive com o builtin
# do Wine (dois arquivos reais no mesmo diretório em filesystem case-sensitive);
# garante que só a cópia nativa exista sob os dois nomes.
cp -f "$S32/ODBC32.dll" "$S32/odbc32.dll" 2>/dev/null || true

# 6) auto-registro de TODOS os componentes (RegisterOCXs dos .inf)
cd "$OLEDB"
for d in oledb32.dll msdaps.dll msdadc.dll msdaenum.dll msdaer.dll msdatt.dll \
         msxactps.dll msdasc.dll msdasql.dll msdaurl.dll sqloledb.dll; do
  [ -f "$d" ] && { wine regsvr32 /s "$d" 2>/dev/null && log "  $d" || log "  $d FALHOU"; }
done
cd "$ADO"
for d in msado15.dll msador15.dll msadrh15.dll msADOX.dll msjro.dll; do
  [ -f "$d" ] && { wine regsvr32 /s "$d" 2>/dev/null && log "  $d" || log "  $d FALHOU"; }
done

# 6b) typelibs do ADO: o regsvr32 registra o caminho com prefixo "\\?\\" que o
#     LoadRegTypeLib do Wine não resolve; e o msado15 builtin procura versões
#     que o MDAC 2.8 não declara. Registrar caminho limpo em todas as versões.
ADOTLB="$WINEPREFIX/drive_c/windows/temp/ado-typelib.reg"
{
  echo 'REGEDIT4'
  echo ''
  for v in 2.0 2.1 2.5 2.6 2.7 2.8 6.0; do
    echo "[HKEY_CLASSES_ROOT\TypeLib\{2A75196C-D9EB-4129-B803-931327F72D5C}\\${v}\\0\\win32]"
    echo '@="C:\\\\Program Files\\\\Common Files\\\\System\\\\ADO\\\\msado15.dll"'
    echo ''
    echo "[HKEY_CLASSES_ROOT\TypeLib\{2A75196C-D9EB-4129-B803-931327F72D5C}\\${v}\\HELPDIR]"
    echo '@="C:\\\\Program Files\\\\Common Files\\\\System\\\\ADO"'
    echo ''
  done
} > "$ADOTLB"
wine regedit /S 'C:\windows\temp\ado-typelib.reg' || log "regedit typelib ADO FALHOU"
log "typelibs ADO ajustadas."

# 7) driver ODBC "SQL Server" + DSN m2master (PristonSQLDll usa DSN=m2master),
#    também via import único (mesma razão do item 5).
DSNFILE="$WINEPREFIX/drive_c/windows/temp/mdac-dsn.reg"
cat > "$DSNFILE" <<EOF
REGEDIT4

[HKEY_LOCAL_MACHINE\\Software\\ODBC\\ODBCINST.INI\\SQL Server]
"Driver"="C:\\\\windows\\\\system32\\\\sqlsrv32.dll"
"Setup"="C:\\\\windows\\\\system32\\\\sqlsrv32.dll"
"APILevel"="2"
"ConnectFunctions"="YYN"
"DriverODBCVer"="03.52"

[HKEY_LOCAL_MACHINE\\Software\\ODBC\\ODBCINST.INI\\ODBC Drivers]
"SQL Server"="Installed"

[HKEY_LOCAL_MACHINE\\Software\\ODBC\\ODBC.INI\\m2master]
"Driver"="C:\\\\windows\\\\system32\\\\sqlsrv32.dll"
"Server"="${SQL_HOST:-127.0.0.1}"
"Database"="accountdb"

[HKEY_LOCAL_MACHINE\\Software\\ODBC\\ODBC.INI\\ODBC Data Sources]
"m2master"="SQL Server"
EOF
wine regedit /S "C:\\windows\\temp\\mdac-dsn.reg" || log "regedit DSN FALHOU rc=$?"
log "DSN m2master registrado."
# NOTA: mesmo registrado corretamente, o subsistema ODBC do Wine 9.0 roteia
# algumas funções (ex.: SQLDriverConnectA) para sua implementação builtin
# incompleta, independente do override "native,builtin" — gap conhecido,
# ver dedicated server/legacy-docker/README.md.
# 8) typelib ADO 2.7 (embutida no msado15.dll) — sem ela CoCreate ADODB dá 8002801D
T='HKLM\\TypeLib\\{00000205-0000-0010-8000-00AA006D2EA4}'
wine reg add "$T\\2.7" /ve /d "Microsoft ActiveX Data Objects 2.7 Library" /f >/dev/null 2>&1
wine reg add "$T\\2.7\\0\\win32" /ve /d 'C:\\Program Files\\Common Files\\System\\ADO\\msado15.dll' /f >/dev/null 2>&1
log "typelib ADO registrada."
log "concluído."