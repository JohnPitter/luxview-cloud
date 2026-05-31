# Bypass do diálogo "Window Mode / FullScreen" do Rakion.
# O load.bin é o RakionLauncher (.NET) empacotado com MPRESS. Aqui nós:
#   1. carregamos o stub load.bin e invocamos o descompressor dele (lf) -> assembly real;
#   2. instanciamos o Form1 INVISÍVEL com o modo escolhido pré-selecionado;
#   3. rodamos a pipeline original (login + decrypt config.xfs + lança rakion.bin +
#      patches do GameGuard) — sem NUNCA mostrar o diálogo.
# Invocado pelo launcher via Windows PowerShell 32-bit (load.bin é x86), elevado.
param(
  [Parameter(Mandatory = $true)][string]$ClientDir,
  [Parameter(Mandatory = $true)][string]$User,
  [Parameter(Mandatory = $true)][string]$HexPass,
  [int]$Windowed = 1
)
$ErrorActionPreference = "Stop"
# Force plain .NET strings — PowerShell param values arrive wrapped (PSObject),
# which fails to bind to the String parameters of the .NET methods below.
$cdir = [string]$ClientDir
$usr = [string]$User
$hex = [string]$HexPass
[System.IO.Directory]::SetCurrentDirectory($cdir)
Add-Type -AssemblyName System.Windows.Forms

$loadBin = [System.IO.Path]::Combine($cdir, "Bin\load.bin")

# 1. unpack: load the MPRESS stub and call its own decompressor (lf) on itself.
$stub = [Reflection.Assembly]::Load([IO.File]::ReadAllBytes($loadBin))
$lf = $stub.GetType("mpress._").GetMethod("lf", [Reflection.BindingFlags]"Static,NonPublic,Public")
$out = [object[]]@($loadBin, $null)
[void]$lf.Invoke($null, $out)
$real = [Reflection.Assembly]::Load([byte[]]$out[1])

# 2. drive Form1 invisibly with the chosen mode pre-selected.
$ft = $real.GetType("RakionLauncher.Form1")
$bf = [Reflection.BindingFlags]"Instance,NonPublic,Public"
$argv = [string[]]@($usr, $hex, "1")
$form = [Activator]::CreateInstance($ft, [object[]]@(, $argv))
$ft.GetField("windowsmode", $bf).SetValue($form, [bool]$Windowed)
$form.Opacity = 0
$form.ShowInTaskbar = $false
$form.WindowState = [System.Windows.Forms.FormWindowState]::Minimized

$startLogin = $ft.GetMethod("StartLogin", $bf)
$form.Add_Shown({ try { $startLogin.Invoke($form, @()) | Out-Null } catch {} })

# 3. run the original pipeline (it Application.Exit()s itself after patching).
[System.Windows.Forms.Application]::Run($form)
