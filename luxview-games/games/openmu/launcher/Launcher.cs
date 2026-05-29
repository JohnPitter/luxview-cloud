// Connection logic adapted from MUnique/OpenMU ClientLauncher (MIT License).
// Writes the server host/port into the registry (the official way up to Season 6)
// and passes /u and /p parameters before starting main.exe.
namespace LuxView.Launcher;

using System.Diagnostics;
using System.Net;
using System.Net.Sockets;
using System.Text;
using System.Windows.Forms;
using Microsoft.Win32;

public class Launcher
{
    public string? HostAddress { get; set; }

    public int HostPort { get; set; }

    public string? MainExePath { get; set; }

    public void LaunchClient()
    {
        if (string.IsNullOrWhiteSpace(this.HostAddress))
        {
            throw new InvalidOperationException("O endereço do servidor não está definido.");
        }

        if (string.IsNullOrWhiteSpace(this.MainExePath) || !File.Exists(this.MainExePath))
        {
            throw new InvalidOperationException($"main.exe não encontrado em '{this.MainExePath}'.");
        }

        if (this.ResolveHost() is not { } ipAddress)
        {
            throw new InvalidOperationException($"Não foi possível resolver '{this.HostAddress}' para um IPv4.");
        }

        if (OperatingSystem.IsWindows())
        {
            using var localMachineKey = RegistryKey.OpenBaseKey(RegistryHive.LocalMachine, RegistryView.Registry32);
            using var key = localMachineKey.CreateSubKey(@"SOFTWARE\WebZen\Mu\Connection");
            key.SetValue("Key", Environment.TickCount, RegistryValueKind.DWord);
            key.SetValue("ParameterA", this.HostEncode(ipAddress), RegistryValueKind.String);
            key.SetValue("ParameterB", this.PortEncode(ipAddress), RegistryValueKind.DWord);
        }

        var info = new DirectoryInfo(this.MainExePath!);
        // UseShellExecute=false launches via CreateProcess directly, avoiding the
        // "Open File - Security Warning"/SmartScreen prompt that ShellExecute shows
        // for downloaded exes (Mark-of-the-Web) — which surfaced as ERROR_CANCELLED.
        // The launcher is already elevated, so main.exe inherits the token.
        var startInfo = new ProcessStartInfo(this.MainExePath, ["connect", $"/u{ipAddress}", $"/p{this.HostPort}"])
        {
            WorkingDirectory = info.Parent!.FullName,
            UseShellExecute = false,
        };

        Process.Start(startInfo);
    }

    private int PortEncode(string ipAddress)
    {
        var port = this.HostPort;
        switch (ipAddress.Length % 4)
        {
            case 0:
                port += 12 - (((port / 4) % 4) * 8);
                return port;
            case 1:
                port += 7 - ((port % 8) * 2);
                return port;
            case 2:
                port += 3 - ((port % 4) * 2);
                return port;
            case 3:
                port += (0x13 - ((port % 4) * 2)) - (((port / 0x10) % 2) * 0x20);
                return port;
            default:
                return port;
        }
    }

    private string? ResolveHost()
    {
        if (IPAddress.TryParse(this.HostAddress, out _))
        {
            return this.HostAddress;
        }

        var entry = Dns.GetHostEntry(this.HostAddress!);
        if (entry.AddressList.FirstOrDefault(a => a.AddressFamily == AddressFamily.InterNetwork) is { } ipAddress)
        {
            var address = ipAddress.ToString();
            // The client blocks 127.0.0.1, so map it to 127.127.127.127.
            return address == "127.0.0.1" ? "127.127.127.127" : address;
        }

        return null;
    }

    private string HostEncode(string ipAddress)
    {
        var result = new StringBuilder();
        var counter = 0;
        foreach (var ch in ipAddress)
        {
            var encodedCharacter = '\0';
            counter++;
            switch (counter)
            {
                case 1:
                    encodedCharacter = (char)(ch + '\f');
                    break;
                case 2:
                    encodedCharacter = (char)(ch + '\a');
                    break;
                case 3:
                    encodedCharacter = (char)(ch + '\x0003');
                    break;
                case 4:
                    encodedCharacter = (char)(ch + '\x0013');
                    counter = 0;
                    break;
                default:
                    break;
            }

            result.Append(encodedCharacter);
        }

        return result.ToString();
    }
}
