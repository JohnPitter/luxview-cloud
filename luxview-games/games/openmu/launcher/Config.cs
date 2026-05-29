namespace LuxView.Launcher;

using System.Text.Json;
using System.Xml.Linq;

/// <summary>
/// Launcher configuration, loaded from files next to the executable:
///  - launcher.config  : OpenMU XML, personalized per server by the engine
///                       (server name / address / port + main.exe path).
///  - luxview-launcher.json : LuxView extras (news URL, patch URLs, title).
/// </summary>
public sealed class Config
{
    public string ServerName { get; set; } = "Servidor LuxView MU";

    public string HostAddress { get; set; } = "127.0.0.1";

    public int HostPort { get; set; } = 44405;

    public string MainExePath { get; set; } = "main.exe";

    public string NewsUrl { get; set; } = "https://mu.luxview.cloud/";

    public string PatchManifestUrl { get; set; } = "https://mu.luxview.cloud/patch/manifest.json";

    public static Config Load(string baseDir)
    {
        var cfg = new Config();

        // Server connection from launcher.config (the engine writes this per server).
        var serverFile = Path.Combine(baseDir, "launcher.config");
        if (File.Exists(serverFile))
        {
            try
            {
                var doc = XDocument.Load(serverFile);
                var mainExe = doc.Descendants("MainExePath").FirstOrDefault()?.Value;
                if (!string.IsNullOrWhiteSpace(mainExe))
                {
                    cfg.MainExePath = mainExe!;
                }

                var host = doc.Descendants("ServerHostSettings").FirstOrDefault();
                if (host is not null)
                {
                    cfg.ServerName = host.Element("Description")?.Value ?? cfg.ServerName;
                    cfg.HostAddress = host.Element("Address")?.Value ?? cfg.HostAddress;
                    if (int.TryParse(host.Element("Port")?.Value, out var port))
                    {
                        cfg.HostPort = port;
                    }
                }
            }
            catch
            {
                // keep defaults on malformed config
            }
        }

        // LuxView extras (news / patch URLs).
        var jsonFile = Path.Combine(baseDir, "luxview-launcher.json");
        if (File.Exists(jsonFile))
        {
            try
            {
                using var stream = File.OpenRead(jsonFile);
                var opts = JsonSerializer.Deserialize<LauncherOptions>(stream, JsonOpts);
                if (opts is not null)
                {
                    if (!string.IsNullOrWhiteSpace(opts.NewsUrl))
                    {
                        cfg.NewsUrl = opts.NewsUrl!;
                    }

                    if (!string.IsNullOrWhiteSpace(opts.PatchManifestUrl))
                    {
                        cfg.PatchManifestUrl = opts.PatchManifestUrl!;
                    }

                    if (!string.IsNullOrWhiteSpace(opts.Title))
                    {
                        cfg.ServerName = opts.Title!;
                    }
                }
            }
            catch
            {
                // keep defaults
            }
        }

        // Resolve main.exe relative to the launcher directory.
        if (!Path.IsPathRooted(cfg.MainExePath))
        {
            cfg.MainExePath = Path.Combine(baseDir, cfg.MainExePath);
        }

        return cfg;
    }

    private static readonly JsonSerializerOptions JsonOpts = new()
    {
        PropertyNameCaseInsensitive = true,
    };

    private sealed class LauncherOptions
    {
        public string? Title { get; set; }

        public string? NewsUrl { get; set; }

        public string? PatchManifestUrl { get; set; }
    }
}
