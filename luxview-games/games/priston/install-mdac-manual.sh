#!/usr/bin/env bash
# Instala MDAC 2.8 SP1 manualmente; o instalador oficial falha no Wine moderno.
set -euo pipefail

MDAC_SRC="${MDAC_SRC:-/opt/mdac/x}"
C="$WINEPREFIX/drive_c"
S32="$C/windows/system32"
OLEDB="$C/Program Files/Common Files/System/OLE DB"
ADO="$C/Program Files/Common Files/System/ADO"
log() { echo "[priston][mdac] $*"; }
mkdir -p "$S32" "$OLEDB" "$ADO" "$C/windows/temp"

cp -f "$MDAC_SRC"/sqloldb/msdart.dll "$S32/"
cp -f "$MDAC_SRC"/sqlnet/{dbnetlib.dll,dbmsgnet.dll,dbmsrpcn.dll,dbnmpntw.dll,cliconfg.dll,cliconfg.rll,sqlunirl.dll} "$S32/"
cp -f "$MDAC_SRC"/sqlodbc/{sqlsrv32.dll,sqlsrv32.rll,odbcbcp.dll} "$S32/"
cp -f "$MDAC_SRC"/mdacxpak/{ODBC32.dll,ODBCINT.dll,ODBCCP32.dll,ODBCCR32.dll,ODBCCU32.dll,ODBCTRAC.dll,ODBC32GT.dll,DS32GT.dll} "$S32/"
cp -f "$MDAC_SRC"/sqloldb/{sqloledb.dll,sqloledb.rll} "$OLEDB/"
cp -f "$MDAC_SRC"/mdacxpak/{oledb32.dll,oledb32a.dll,oledb32r.dll,msdaps.dll,msxactps.dll,msdadc.dll,msdaenum.dll,msdaer.dll,msdaurl.dll,msdatt.dll,msdasql.dll,msdasqlr.dll,msdasc.dll,msdatl3.dll,simpdata.tlb} "$OLEDB/"
cp -f "$MDAC_SRC"/mdacxpak/{msado15.dll,msador15.dll,msader15.dll,msadrh15.dll,msADOX.dll,msjro.dll,msdatsrc.tlb,msado20.tlb,msado21.tlb,msado25.tlb,msado26.tlb,msado27.tlb} "$ADO/"
for f in sqloledb.dll sqloledb.rll oledb32.dll oledb32a.dll oledb32r.dll msdaps.dll msdaenum.dll msdaer.dll msdadc.dll msdatt.dll msdatl3.dll msxactps.dll msdasql.dll msdasqlr.dll msdasc.dll msado15.dll msador15.dll msader15.dll msadrh15.dll msdaosp.dll; do rm -f "$S32/$f"; done

regfile="$C/windows/temp/mdac-overrides.reg"
{
 echo 'REGEDIT4'; echo
 echo '[HKEY_LOCAL_MACHINE\Software\Wine\DllOverrides]'
 for dll in sqloledb oledb32 oledb32a msdasc msdaps msdaenum msdaer msdadc msdatt msdatl3 msxactps msdasql msado15 msador15 msadox msjro odbc32 odbccp32 odbcint odbccr32 odbccu32 odbctrac sqlsrv32 dbnetlib dbmsgnet dbmsrpcn dbnmpntw cliconfg sqlunirl msdart; do printf '"%s"="native,builtin"\n' "$dll"; done
} > "$regfile"
wine regedit /S 'C:\windows\temp\mdac-overrides.reg'
cp -f "$S32/ODBC32.dll" "$S32/odbc32.dll"
for d in oledb32.dll msdaps.dll msdadc.dll msdaenum.dll msdaer.dll msdatt.dll msxactps.dll msdasc.dll msdasql.dll msdaurl.dll sqloledb.dll; do [ -f "$OLEDB/$d" ] && wine regsvr32 /s "$OLEDB/$d" || log "$d registro falhou"; done
for d in msado15.dll msador15.dll msadrh15.dll msADOX.dll msjro.dll; do [ -f "$ADO/$d" ] && wine regsvr32 /s "$ADO/$d" || log "$d registro falhou"; done

tlb="$C/windows/temp/ado-typelib.reg"
{
 echo 'REGEDIT4'; echo
 for v in 2.0 2.1 2.5 2.6 2.7 2.8 6.0; do
  echo "[HKEY_CLASSES_ROOT\TypeLib\{2A75196C-D9EB-4129-B803-931327F72D5C}\$v\0\win32]"
  echo '@="C:\\\\Program Files\\\\Common Files\\\\System\\\\ADO\\\\msado15.dll"'; echo
  echo "[HKEY_CLASSES_ROOT\TypeLib\{2A75196C-D9EB-4129-B803-931327F72D5C}\$v\HELPDIR]"
  echo '@="C:\\\\Program Files\\\\Common Files\\\\System\\\\ADO"'; echo
 done
} > "$tlb"
wine regedit /S 'C:\windows\temp\ado-typelib.reg'

# sqlsrv32 driver and m2master DSN used by PristonSQLDll.dll.
dsn="$C/windows/temp/mdac-dsn.reg"
cat > "$dsn" <<EOF
REGEDIT4

[HKEY_LOCAL_MACHINE\\Software\\ODBC\\ODBCINST.INI\\SQL Server]
"Driver"="C:\\\\windows\\\\system32\\sqlsrv32.dll"
"Setup"="C:\\\\windows\\\\system32\\sqlsrv32.dll"
"APILevel"="2"
"ConnectFunctions"="YYN"
"DriverODBCVer"="03.52"

[HKEY_LOCAL_MACHINE\\Software\\ODBC\\ODBCINST.INI\\ODBC Drivers]
"SQL Server"="Installed"

[HKEY_LOCAL_MACHINE\\Software\\ODBC\\ODBC.INI\\m2master]
"Driver"="C:\\\\windows\\\\system32\\sqlsrv32.dll"
"Server"="127.0.0.1,1433"
"Database"="accountdb"

[HKEY_LOCAL_MACHINE\\Software\\ODBC\\ODBC.INI\\ODBC Data Sources]
"m2master"="SQL Server"
EOF
wine regedit /S 'C:\windows\temp\mdac-dsn.reg'
log 'MDAC, overrides, typelibs e DSN SQL Server registrados.'
