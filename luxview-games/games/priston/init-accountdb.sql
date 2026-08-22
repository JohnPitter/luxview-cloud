IF DB_ID(N'accountdb') IS NULL
    CREATE DATABASE [accountdb];
GO
USE [accountdb];
GO
DECLARE @letter CHAR(1) = 'A';
WHILE @letter <= 'Z'
BEGIN
    DECLARE @sql nvarchar(max) = N'
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
    SET @letter = CHAR(ASCII(@letter) + 1);
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
