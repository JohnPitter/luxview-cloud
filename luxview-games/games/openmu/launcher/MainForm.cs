namespace LuxView.Launcher;

using System.Drawing;
using System.Windows.Forms;
using Microsoft.Web.WebView2.WinForms;

public sealed class MainForm : Form
{
    private static readonly Color Bg = Color.FromArgb(14, 14, 16);
    private static readonly Color Panel = Color.FromArgb(22, 22, 24);
    private static readonly Color Accent = Color.FromArgb(245, 166, 35);
    private static readonly Color Fg = Color.FromArgb(230, 230, 230);
    private static readonly Color Muted = Color.FromArgb(150, 150, 150);

    private readonly Config _config;
    private readonly WebView2 _news = new();
    private readonly Button _playButton = new();
    private readonly ProgressBar _progress = new();
    private readonly Label _status = new();
    private bool _ready;

    public MainForm()
    {
        var baseDir = AppContext.BaseDirectory;
        _config = Config.Load(baseDir);

        this.Text = _config.ServerName;
        this.BackColor = Bg;
        this.ForeColor = Fg;
        this.FormBorderStyle = FormBorderStyle.FixedSingle;
        this.MaximizeBox = false;
        this.StartPosition = FormStartPosition.CenterScreen;
        this.ClientSize = new Size(960, 620);
        this.Font = new Font("Segoe UI", 9F);

        BuildHeader();
        BuildFooter();
        BuildNews();

        this.Load += OnLoad;
    }

    private void BuildHeader()
    {
        var header = new Panel { Dock = DockStyle.Top, Height = 64, BackColor = Panel };
        var title = new Label
        {
            Text = _config.ServerName,
            ForeColor = Accent,
            Font = new Font("Segoe UI", 16F, FontStyle.Bold),
            AutoSize = false,
            Dock = DockStyle.Fill,
            TextAlign = ContentAlignment.MiddleLeft,
            Padding = new Padding(20, 0, 0, 0),
        };
        var addr = new Label
        {
            Text = $"{_config.HostAddress}:{_config.HostPort}",
            ForeColor = Muted,
            Font = new Font("Consolas", 10F),
            AutoSize = false,
            Dock = DockStyle.Right,
            Width = 220,
            TextAlign = ContentAlignment.MiddleRight,
            Padding = new Padding(0, 0, 20, 0),
        };
        header.Controls.Add(title);
        header.Controls.Add(addr);
        this.Controls.Add(header);
    }

    private void BuildFooter()
    {
        var footer = new Panel { Dock = DockStyle.Bottom, Height = 96, BackColor = Panel };

        _progress.Dock = DockStyle.Top;
        _progress.Height = 6;
        _progress.Style = ProgressBarStyle.Continuous;

        _status.Text = "Iniciando...";
        _status.ForeColor = Muted;
        _status.AutoSize = false;
        _status.Dock = DockStyle.Fill;
        _status.TextAlign = ContentAlignment.MiddleLeft;
        _status.Padding = new Padding(20, 0, 0, 0);

        _playButton.Text = "JOGAR";
        _playButton.Enabled = false;
        _playButton.Font = new Font("Segoe UI", 14F, FontStyle.Bold);
        _playButton.ForeColor = Color.Black;
        _playButton.BackColor = Accent;
        _playButton.FlatStyle = FlatStyle.Flat;
        _playButton.FlatAppearance.BorderSize = 0;
        _playButton.Size = new Size(180, 60);
        _playButton.Dock = DockStyle.Right;
        _playButton.Cursor = Cursors.Hand;
        _playButton.Click += OnPlay;

        footer.Controls.Add(_status);
        footer.Controls.Add(_playButton);
        footer.Controls.Add(_progress);
        this.Controls.Add(footer);
    }

    private void BuildNews()
    {
        _news.Dock = DockStyle.Fill;
        _news.DefaultBackgroundColor = Bg;
        this.Controls.Add(_news);
        _news.BringToFront();
    }

    private async void OnLoad(object? sender, EventArgs e)
    {
        // News panel — degrade gracefully if the WebView2 runtime is missing/offline.
        try
        {
            await _news.EnsureCoreWebView2Async().ConfigureAwait(true);
            _news.CoreWebView2.Navigate(_config.NewsUrl);
        }
        catch
        {
            _news.Visible = false;
        }

        // Auto-update check.
        var progress = new Progress<UpdateStatus>(s =>
        {
            _status.Text = s.Message;
            _progress.Value = Math.Clamp(s.Percent, 0, 100);
        });

        bool ok;
        try
        {
            ok = await new Updater().RunAsync(AppContext.BaseDirectory, _config.PatchManifestUrl, progress, CancellationToken.None).ConfigureAwait(true);
        }
        catch
        {
            ok = true; // never block playing on updater errors
            _status.Text = "Você pode jogar.";
            _progress.Value = 100;
        }

        _ready = ok;
        _playButton.Enabled = true;
        _playButton.Text = ok ? "JOGAR" : "TENTAR NOVAMENTE";
    }

    private void OnPlay(object? sender, EventArgs e)
    {
        if (!_ready)
        {
            // Retry the update check.
            _playButton.Enabled = false;
            OnLoad(this, EventArgs.Empty);
            return;
        }

        try
        {
            new Launcher
            {
                HostAddress = _config.HostAddress,
                HostPort = _config.HostPort,
                MainExePath = _config.MainExePath,
            }.LaunchClient();

            this.WindowState = FormWindowState.Minimized;
        }
        catch (Exception ex)
        {
            MessageBox.Show(ex.Message, "Não foi possível iniciar o jogo", MessageBoxButtons.OK, MessageBoxIcon.Error);
        }
    }
}
