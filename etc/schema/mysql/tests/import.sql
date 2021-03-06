-- IcingaDB | (c) 2019 Icinga GmbH | GPLv2+

LOAD DATA INFILE '/var/lib/mysql-files/environment.csv'
REPLACE
INTO TABLE environment
COLUMNS TERMINATED BY ';'
OPTIONALLY ENCLOSED BY '"'
ESCAPED BY '"'
LINES TERMINATED BY '\n'
IGNORE 1 LINES;

LOAD DATA INFILE '/var/lib/mysql-files/host.csv'
REPLACE
INTO TABLE host
CHARACTER SET utf8mb4
COLUMNS TERMINATED BY ';'
OPTIONALLY ENCLOSED BY '"'
ESCAPED BY '"'
LINES TERMINATED BY '\n'
IGNORE 1 LINES;

LOAD DATA INFILE '/var/lib/mysql-files/hostgroup.csv'
REPLACE
INTO TABLE hostgroup
COLUMNS TERMINATED BY ';'
OPTIONALLY ENCLOSED BY '"'
ESCAPED BY '"'
LINES TERMINATED BY '\n'
IGNORE 1 LINES;

LOAD DATA INFILE '/var/lib/mysql-files/hostgroup_member.csv'
REPLACE
INTO TABLE hostgroup_member
COLUMNS TERMINATED BY ';'
OPTIONALLY ENCLOSED BY '"'
ESCAPED BY '"'
LINES TERMINATED BY '\n'
IGNORE 1 LINES;

LOAD DATA INFILE '/var/lib/mysql-files/service.csv'
REPLACE
INTO TABLE service
CHARACTER SET utf8mb4
COLUMNS TERMINATED BY ';'
OPTIONALLY ENCLOSED BY '"'
ESCAPED BY '"'
LINES TERMINATED BY '\n'
IGNORE 1 LINES;

LOAD DATA INFILE '/var/lib/mysql-files/servicegroup.csv'
REPLACE
INTO TABLE servicegroup
COLUMNS TERMINATED BY ';'
OPTIONALLY ENCLOSED BY '"'
ESCAPED BY '"'
LINES TERMINATED BY '\n'
IGNORE 1 LINES;

LOAD DATA INFILE '/var/lib/mysql-files/servicegroup_member.csv'
REPLACE
INTO TABLE servicegroup_member
COLUMNS TERMINATED BY ';'
OPTIONALLY ENCLOSED BY '"'
ESCAPED BY '"'
LINES TERMINATED BY '\n'
IGNORE 1 LINES;

LOAD DATA INFILE '/var/lib/mysql-files/command.csv'
REPLACE
INTO TABLE command
COLUMNS TERMINATED BY ';'
OPTIONALLY ENCLOSED BY '"'
ESCAPED BY '"'
LINES TERMINATED BY '\n'
IGNORE 1 LINES;

LOAD DATA INFILE '/var/lib/mysql-files/eventcommand.csv'
REPLACE
INTO TABLE eventcommand
COLUMNS TERMINATED BY ';'
OPTIONALLY ENCLOSED BY '"'
ESCAPED BY '"'
LINES TERMINATED BY '\n'
IGNORE 1 LINES;

LOAD DATA INFILE '/var/lib/mysql-files/action_url.csv'
REPLACE
INTO TABLE action_url
COLUMNS TERMINATED BY ';'
OPTIONALLY ENCLOSED BY '"'
ESCAPED BY '"'
LINES TERMINATED BY '\n'
IGNORE 1 LINES;

LOAD DATA INFILE '/var/lib/mysql-files/notes_url.csv'
REPLACE
INTO TABLE notes_url
COLUMNS TERMINATED BY ';'
OPTIONALLY ENCLOSED BY '"'
ESCAPED BY '"'
LINES TERMINATED BY '\n'
IGNORE 1 LINES;

LOAD DATA INFILE '/var/lib/mysql-files/icon_image.csv'
REPLACE
INTO TABLE icon_image
COLUMNS TERMINATED BY ';'
OPTIONALLY ENCLOSED BY '"'
ESCAPED BY '"'
LINES TERMINATED BY '\n'
IGNORE 1 LINES;

LOAD DATA INFILE '/var/lib/mysql-files/endpoint.csv'
REPLACE
INTO TABLE endpoint
COLUMNS TERMINATED BY ';'
OPTIONALLY ENCLOSED BY '"'
ESCAPED BY '"'
LINES TERMINATED BY '\n'
IGNORE 1 LINES;

LOAD DATA INFILE '/var/lib/mysql-files/zone.csv'
REPLACE
INTO TABLE zone
COLUMNS TERMINATED BY ';'
OPTIONALLY ENCLOSED BY '"'
ESCAPED BY '"'
LINES TERMINATED BY '\n'
IGNORE 1 LINES;

LOAD DATA INFILE '/var/lib/mysql-files/notification.csv'
REPLACE
INTO TABLE notification
CHARACTER SET utf8mb4
COLUMNS TERMINATED BY ';'
OPTIONALLY ENCLOSED BY '"'
ESCAPED BY '"'
LINES TERMINATED BY '\n'
IGNORE 1 LINES;

LOAD DATA INFILE '/var/lib/mysql-files/user.csv'
REPLACE
INTO TABLE user
CHARACTER SET utf8mb4
COLUMNS TERMINATED BY ';'
OPTIONALLY ENCLOSED BY '"'
ESCAPED BY '"'
LINES TERMINATED BY '\n'
IGNORE 1 LINES;

LOAD DATA INFILE '/var/lib/mysql-files/usergroup.csv'
REPLACE
INTO TABLE usergroup
COLUMNS TERMINATED BY ';'
OPTIONALLY ENCLOSED BY '"'
ESCAPED BY '"'
LINES TERMINATED BY '\n'
IGNORE 1 LINES;

LOAD DATA INFILE '/var/lib/mysql-files/usergroup_member.csv'
REPLACE
INTO TABLE usergroup_member
COLUMNS TERMINATED BY ';'
OPTIONALLY ENCLOSED BY '"'
ESCAPED BY '"'
LINES TERMINATED BY '\n'
IGNORE 1 LINES;