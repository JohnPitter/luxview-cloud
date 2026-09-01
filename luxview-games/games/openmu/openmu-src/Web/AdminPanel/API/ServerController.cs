// <copyright file="ServerController.cs" company="MUnique">
// Licensed under the MIT License. See LICENSE file in the project root for full license information.
// </copyright>

namespace MUnique.OpenMU.Web.API
{
    using Microsoft.AspNetCore.Mvc;
    using MUnique.OpenMU.DataModel.Entities;
    using MUnique.OpenMU.GameLogic;
    using MUnique.OpenMU.GameServer;
    using MUnique.OpenMU.Interfaces;
    using MUnique.OpenMU.Persistence;

    /// <summary>
    /// Server API controller.
    /// </summary>
    [Route("api/")]
    public class ServerController : Controller
    {
        private IDictionary<int, IGameServer> _gameServers;

        /// <summary>
        /// Initializes a new instance of the <see cref="ServerController"/> class.
        /// </summary>
        /// <param name="gameServers">The game servers.</param>
        public ServerController(IDictionary<int, IGameServer> gameServers) => this._gameServers = gameServers;

        /// <summary>
        /// Sends a global message to the specified server.
        /// </summary>
        /// <param name="id">The server id.</param>
        /// <param name="msg">The message.</param>
        [Route("send/{id=0}")]
        public async Task<IActionResult> SendGlobalMessageAsync(int id, [FromQuery(Name = "msg")] string msg)
        {
            var server = (GameServer)this._gameServers.Values.ElementAt(id);
            if (server is not null)
            {
                await server.Context.SendGlobalNotificationAsync(msg).ConfigureAwait(false);
                return this.Ok("Done");
            }

            return this.Ok("Server not ready");
        }

        /// <summary>
        /// Gets a flag, if the specified account is currently online.
        /// </summary>
        /// <param name="accountName">Name of the account.</param>
        /// <returns>True, when online.</returns>
        [HttpGet]
        [Route("is-online/{accountName=0}")]
        public async Task<bool> GetIsOnlineAsync(string accountName)
        {
            var isOnline = false;

            foreach (var server in this._gameServers.Values.OfType<GameServer>())
            {
                var players = await server.Context.GetPlayersAsync().ConfigureAwait(false);
                if (players.Any(p => p.Account?.LoginName == accountName))
                {
                    isOnline = true;
                    break;
                }
            }

            return isOnline;
        }

        /// <summary>
        /// Gets the server state.
        /// </summary>
        [HttpGet]
        [Route("status")]
        public async Task<IActionResult> ServerStateAsync()
        {
            var list = new List<string>();
            var sum = 0;
            foreach (var server in this._gameServers.Values.OfType<GameServer>())
            {
                sum += server.Context.PlayerCount;
                await server.Context.ForEachPlayerAsync(player =>
                {
                    if (!string.IsNullOrWhiteSpace(player.Name))
                    {
                        list.Add(player.Name);
                    }

                    return Task.CompletedTask;
                }).ConfigureAwait(false);
            }

            var item = new
            {
                state = "Online",
                players = sum,
                playersList = list,
            };

            return this.Ok(item);
        }

        /// <summary>
        /// Live roster of characters with an active GameServer session (LuxView dashboard).
        /// </summary>
        [HttpGet]
        [Route("players")]
        public async Task<IActionResult> GetOnlinePlayersAsync()
        {
            var players = new List<object>();
            foreach (var server in this._gameServers.Values.OfType<GameServer>())
            {
                await server.Context.ForEachPlayerAsync(player =>
                {
                    if (!player.IsConnected || player.SelectedCharacter is null)
                    {
                        return Task.CompletedTask;
                    }

                    if (player.Account is { IsBot: true } or { IsTemplate: true })
                    {
                        return Task.CompletedTask;
                    }

                    var character = player.SelectedCharacter;
                    var mapName = player.CurrentMap?.Definition.Name
                        ?? character.CurrentMap?.Name
                        ?? string.Empty;
                    players.Add(new
                    {
                        character = player.Name,
                        account = player.Account?.LoginName ?? string.Empty,
                        @class = character.CharacterClass?.Name ?? string.Empty,
                        level = player.Level,
                        location = mapName,
                        server_id = server.Id,
                        server_description = server.Description,
                    });
                    return Task.CompletedTask;
                }).ConfigureAwait(false);
            }

            return this.Ok(new { players });
        }
    }
}