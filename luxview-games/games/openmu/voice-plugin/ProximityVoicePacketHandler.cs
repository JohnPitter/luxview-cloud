namespace LuxView.OpenMU.Voice;

using System.Buffers;
using System.Buffers.Binary;
using System.Diagnostics;
using System.Reflection;
using System.Runtime.CompilerServices;
using System.Runtime.InteropServices;
using MUnique.OpenMU.GameLogic;
using MUnique.OpenMU.GameServer.MessageHandler;
using MUnique.OpenMU.GameServer.RemoteView;
using MUnique.OpenMU.Network;
using MUnique.OpenMU.PlugIns;

/// <summary>
/// Relays 20 ms IMA ADPCM voice frames to authenticated players in a
/// server-authoritative 10-tile circle on the same map.
/// </summary>
[PlugIn]
[Guid("9A2C5426-F659-44A7-9D03-BE4875972932")]
public sealed class ProximityVoicePacketHandler : IPacketHandlerPlugIn
{
    /// <inheritdoc />
    public bool IsEncryptionExpected => false;

    /// <inheritdoc />
    public byte Key => VoicePacket.Code;

    /// <inheritdoc />
    public async ValueTask HandlePacketAsync(Player player, Memory<byte> packet)
    {
        if (player is not RemotePlayer speaker || !VoicePacket.IsValidClientFrame(packet.Span))
        {
            return;
        }

        if (!VoiceRateLimiter.TryAcquire(player))
        {
            return;
        }

        var frame = ArrayPool<byte>.Shared.Rent(VoicePacket.EncodedFrameBytes);
        try
        {
            packet.Span.Slice(VoicePacket.ClientHeaderBytes, VoicePacket.EncodedFrameBytes).CopyTo(frame);
            await VoicePacket.RelayAsync(speaker, frame).ConfigureAwait(false);
        }
        finally
        {
            ArrayPool<byte>.Shared.Return(frame);
        }
    }
}

internal static class VoicePacket
{
    internal const byte Code = 0xF4;
    internal const byte FrameSubCode = 0x20;
    internal const int ClientHeaderBytes = 4;
    internal const int EncodedFrameBytes = 83;
    internal const int ClientPacketBytes = ClientHeaderBytes + EncodedFrameBytes;
    internal const int ServerPacketBytes = ClientHeaderBytes + sizeof(ushort) + EncodedFrameBytes;
    private const int ProximityRadius = 10;
    private const int ProximityRadiusSquared = ProximityRadius * ProximityRadius;
    private static readonly PropertyInfo? ConnectionProperty = typeof(RemotePlayer).GetProperty("Connection", BindingFlags.Instance | BindingFlags.NonPublic);

    internal static bool IsValidClientFrame(ReadOnlySpan<byte> packet)
    {
        return packet.Length == ClientPacketBytes
            && packet[0] == 0xC1
            && packet[1] == ClientPacketBytes
            && packet[2] == Code
            && packet[3] == FrameSubCode;
    }

    internal static async ValueTask RelayAsync(RemotePlayer speaker, byte[] frame)
    {
        var map = speaker.CurrentMap;
        if (map is null)
        {
            return;
        }

        foreach (var recipient in map.GetAttackablesInRange(speaker.Position, ProximityRadius).OfType<RemotePlayer>())
        {
            if (recipient.Id == speaker.Id || !IsWithinProximity(speaker, recipient))
            {
                continue;
            }

            var connection = GetConnection(recipient);
            if (connection is null)
            {
                continue;
            }

            await SendFrameAsync(connection, speaker.Id, frame).ConfigureAwait(false);
        }
    }

    private static IConnection? GetConnection(RemotePlayer player)
    {
        return ConnectionProperty?.GetValue(player) as IConnection;
    }

    private static bool IsWithinProximity(Player speaker, Player recipient)
    {
        var x = speaker.Position.X - recipient.Position.X;
        var y = speaker.Position.Y - recipient.Position.Y;
        return (x * x) + (y * y) <= ProximityRadiusSquared;
    }

    private static ValueTask SendFrameAsync(IConnection connection, ushort senderId, byte[] frame)
    {
        return connection.SendAsync(() =>
        {
            var packet = connection.Output.GetSpan(ServerPacketBytes)[..ServerPacketBytes];
            packet[0] = 0xC1;
            packet[1] = ServerPacketBytes;
            packet[2] = Code;
            packet[3] = FrameSubCode;
            BinaryPrimitives.WriteUInt16LittleEndian(packet.Slice(ClientHeaderBytes, sizeof(ushort)), senderId);
            frame.AsSpan(0, EncodedFrameBytes).CopyTo(packet[(ClientHeaderBytes + sizeof(ushort))..]);
            return ServerPacketBytes;
        });
    }
}

internal static class VoiceRateLimiter
{
    private const int MaxFramesPerSecond = 60;
    private static readonly ConditionalWeakTable<Player, VoiceRateWindow> Windows = new();

    internal static bool TryAcquire(Player player)
    {
        return Windows.GetValue(player, _ => new VoiceRateWindow()).TryAcquire();
    }

    private sealed class VoiceRateWindow
    {
        private readonly object _lock = new();
        private long _windowStart = Stopwatch.GetTimestamp();
        private int _frames;

        internal bool TryAcquire()
        {
            lock (this._lock)
            {
                if (Stopwatch.GetElapsedTime(this._windowStart) >= TimeSpan.FromSeconds(1))
                {
                    this._windowStart = Stopwatch.GetTimestamp();
                    this._frames = 0;
                }

                if (this._frames >= MaxFramesPerSecond)
                {
                    return false;
                }

                this._frames++;
                return true;
            }
        }
    }
}
