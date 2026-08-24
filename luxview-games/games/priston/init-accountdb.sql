IF DB_ID(N'accountdb') IS NULL
    CREATE DATABASE [accountdb];
GO
-- O executável abre estes catálogos mesmo quando as tabelas de billing/log estão vazias.
-- Criação condicional mantém o bootstrap seguro para reexecuções.
IF DB_ID(N'BillingDb') IS NULL CREATE DATABASE [BillingDb];
GO
IF DB_ID(N'BillingLogDb') IS NULL CREATE DATABASE [BillingLogDb];
GO
IF DB_ID(N'GameLogDb') IS NULL CREATE DATABASE [GameLogDb];
GO
IF DB_ID(N'PCRoom') IS NULL CREATE DATABASE [PCRoom];
GO
IF DB_ID(N'PCRoomLog') IS NULL CREATE DATABASE [PCRoomLog];
GO
IF DB_ID(N'ItemLogDb') IS NULL CREATE DATABASE [ItemLogDb];
GO
IF DB_ID(N'ClanDb') IS NULL CREATE DATABASE [ClanDb];
GO
IF DB_ID(N'Sod2Db') IS NULL CREATE DATABASE [Sod2Db];
GO
USE [accountdb];
GO
-- Explicit A-Z: a WHILE CHAR(ASCII('Z')+1) becomes '[' and creates a bogus [GameUser table.
DECLARE @sql nvarchar(max);
DECLARE @letter char(1);
DECLARE @i int = 0;
WHILE @i < 26
BEGIN
    SET @letter = CHAR(65 + @i);
    SET @sql = N'
IF OBJECT_ID(N''dbo.' + @letter + N'GameUser'', N''U'') IS NULL
BEGIN
    CREATE TABLE [dbo].[' + @letter + N'GameUser] (
        [userid] varchar(16) NOT NULL PRIMARY KEY,
        [Passwd] varchar(32) NOT NULL,
        [GameCode] varchar(32) NULL,
        [GPCode] varchar(32) NULL,
        [RegistDay] datetime NULL,
        [DisuseDay] datetime NULL,
        [UsePeriod] int NULL,
        [Credit] int NULL,
        [SelectChk] int NULL,
        [EventChk] int NULL,
        [BlockChk] int NULL,
        [inuse] int NULL,
        [DelChk] int NULL,
        [ServerName] varchar(32) NULL,
        [EditDay] datetime NULL,
        [RNo] int NULL,
        [SNo] varchar(32) NULL,
        [Channel] varchar(50) NULL,
        [BNum] int NULL,
        [MXServer] varchar(32) NULL,
        [MXChar] varchar(32) NULL,
        [MXType] int NULL,
        [MXLevel] int NULL,
        [MXExp] bigint NULL,
        [SpecialChk] int NULL,
        [ECoin] int NULL,
        [StartDay] datetime NULL,
        [LastDay] datetime NULL
    );
END';
    EXEC (@sql);
    SET @i = @i + 1;
END;
GO
IF OBJECT_ID(N'dbo.FPersonalMember', N'U') IS NULL
BEGIN
    CREATE TABLE [dbo].[FPersonalMember] (
        [userid] varchar(16) NOT NULL PRIMARY KEY,
        [Channel] varchar(50) NULL,
        [BNum] int NULL
    );
END;
GO
