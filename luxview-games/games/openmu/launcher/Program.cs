namespace LuxView.Launcher;

using System.Windows.Forms;

internal static class Program
{
    [STAThread]
    internal static void Main()
    {
        Application.EnableVisualStyles();
        Application.SetCompatibleTextRenderingDefault(false);
        Application.Run(new MainForm());
    }
}
