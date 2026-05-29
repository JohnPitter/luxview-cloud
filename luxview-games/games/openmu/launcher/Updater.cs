namespace LuxView.Launcher;

using System.Security.Cryptography;
using System.Text.Json;
using System.Text.Json.Serialization;

public sealed record UpdateStatus(string Message, int Percent);

public sealed class Updater
{
    private static readonly HttpClient Http = new() { Timeout = TimeSpan.FromMinutes(30) };

    /// <summary>
    /// Checks the remote manifest and downloads any file whose local SHA-256 differs.
    /// Network failures are non-fatal: the player can still launch with the current files.
    /// </summary>
    /// <returns>True if the client is ready to play (always true unless a download failed).</returns>
    public async Task<bool> RunAsync(string baseDir, string manifestUrl, IProgress<UpdateStatus> progress, CancellationToken ct)
    {
        progress.Report(new("Verificando atualizações...", 0));

        Manifest? manifest;
        try
        {
            await using var stream = await Http.GetStreamAsync(manifestUrl, ct).ConfigureAwait(false);
            manifest = await JsonSerializer.DeserializeAsync<Manifest>(stream, JsonOpts, ct).ConfigureAwait(false);
        }
        catch
        {
            progress.Report(new("Não foi possível verificar atualizações (offline?). Você ainda pode jogar.", 100));
            return true;
        }

        var files = manifest?.Files ?? [];
        if (files.Count == 0)
        {
            progress.Report(new("Cliente atualizado. Pronto pra jogar!", 100));
            return true;
        }

        var baseUri = new Uri(manifestUrl);
        var pending = new List<PatchFile>();
        for (var i = 0; i < files.Count; i++)
        {
            ct.ThrowIfCancellationRequested();
            var f = files[i];
            progress.Report(new($"Verificando arquivos... ({i + 1}/{files.Count})", (int)((i + 1) * 100.0 / files.Count / 2)));
            var local = Path.Combine(baseDir, f.Path.Replace('/', Path.DirectorySeparatorChar));
            if (!File.Exists(local) || !HashMatches(local, f.Sha256))
            {
                pending.Add(f);
            }
        }

        if (pending.Count == 0)
        {
            progress.Report(new("Cliente atualizado. Pronto pra jogar!", 100));
            return true;
        }

        for (var i = 0; i < pending.Count; i++)
        {
            ct.ThrowIfCancellationRequested();
            var f = pending[i];
            progress.Report(new($"Baixando atualização {i + 1}/{pending.Count}: {f.Path}", 50 + (int)(i * 50.0 / pending.Count)));
            var url = !string.IsNullOrWhiteSpace(f.Url) ? f.Url! : new Uri(baseUri, "patch/" + f.Path).ToString();
            var dest = Path.Combine(baseDir, f.Path.Replace('/', Path.DirectorySeparatorChar));
            try
            {
                Directory.CreateDirectory(Path.GetDirectoryName(dest)!);
                var tmp = dest + ".part";
                await using (var resp = await Http.GetStreamAsync(url, ct).ConfigureAwait(false))
                await using (var file = File.Create(tmp))
                {
                    await resp.CopyToAsync(file, ct).ConfigureAwait(false);
                }

                File.Move(tmp, dest, overwrite: true);
            }
            catch
            {
                progress.Report(new($"Falha ao baixar {f.Path}. Tente novamente mais tarde.", 100));
                return false;
            }
        }

        progress.Report(new("Atualização concluída. Pronto pra jogar!", 100));
        return true;
    }

    private static bool HashMatches(string path, string? expected)
    {
        if (string.IsNullOrWhiteSpace(expected))
        {
            return true;
        }

        try
        {
            using var sha = SHA256.Create();
            using var stream = File.OpenRead(path);
            var hash = Convert.ToHexString(sha.ComputeHash(stream));
            return string.Equals(hash, expected, StringComparison.OrdinalIgnoreCase);
        }
        catch
        {
            return false;
        }
    }

    private static readonly JsonSerializerOptions JsonOpts = new() { PropertyNameCaseInsensitive = true };

    private sealed class Manifest
    {
        [JsonPropertyName("version")]
        public string? Version { get; set; }

        [JsonPropertyName("files")]
        public List<PatchFile> Files { get; set; } = [];
    }

    private sealed class PatchFile
    {
        [JsonPropertyName("path")]
        public string Path { get; set; } = string.Empty;

        [JsonPropertyName("sha256")]
        public string? Sha256 { get; set; }

        [JsonPropertyName("url")]
        public string? Url { get; set; }
    }
}
