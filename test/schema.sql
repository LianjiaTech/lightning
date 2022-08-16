SET NAMES UTF8MB4;

CREATE DATABASE IF NOT EXISTS test;
USE test;

DROP TABLE IF EXISTS `tb`;
CREATE TABLE `tb` (
  `a` int(11) NOT NULL,
  `b` varchar(10) DEFAULT NULL,
  PRIMARY KEY (`a`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

INSERT INTO `tb` VALUES (1, "abc");
INSERT INTO `tb` VALUES (2, "ghi");
INSERT INTO `tb` VALUES (3, 'space ');
UPDATE `tb` SET b = "中文" WHERE a = 2;
UPDATE `tb` SET b = "'abc'" WHERE a = 2;
UPDATE `tb` SET b = '"abc' WHERE a = 2;
DELETE FROM `tb` WHERE a = 1;

DROP TABLE IF EXISTS `setTest`;
CREATE TABLE `setTest` (
  id int(11) AUTO_INCREMENT,
  attrib SET('bold','italic','underline'),
  PRIMARY KEY (`id`)
);

INSERT INTO setTest (attrib) VALUES ('bold');
INSERT INTO setTest (attrib) VALUES ('bold,italic');
INSERT INTO setTest (attrib) VALUES ('bold,italic,underline');

DROP TABLE IF EXISTS `enumTest`;
CREATE TABLE `enumTest` (
  id int(11) AUTO_INCREMENT,
  color ENUM('red','green','blue'),
  PRIMARY KEY (`id`) 
);

INSERT INTO `enumTest` (color) VALUES ('red');

DROP TABLE IF EXISTS `bitTest`;
CREATE TABLE `bitTest` (
  id int(11) AUTO_INCREMENT,
  days BIT(7),
  PRIMARY KEY(id)
);

INSERT INTO `bitTest` (`days`) VALUES (B'1111100');

CREATE TABLE testNoPRI (
  `a` int,
  `b` varchar(10)
);

INSERT INTO testNoPRI VALUES (1, 'abc');

CREATE TABLE `timeTest` (
  `a` timestamp NULL DEFAULT NULL,
  `b` datetime DEFAULT NULL
) ENGINE=InnoDB;

INSERT INTO timeTest VALUES ("2016-06-01 23:55:29", "2016-06-01 23:55:29");

CREATE TABLE testDecimal(
  `d1` DECIMAL(38,2) NOT NULL DEFAULT 0,
  `d2` DECIMAL(6,2) NOT NULL DEFAULT 0
) ENGINE=InnoDB;

INSERT INTO testDecimal VALUES (2011202301003814000564, 2.4);
INSERT INTO testDecimal VALUES (2011202301003814000564.2, 24);

CREATE TABLE `test_int_max` (
  `a` TINYINT unsigned NOT NULL,
  `b` SMALLINT unsigned NOT NULL,
  `c` MEDIUMINT unsigned NOT NULL,
  `d` INT unsigned NOT NULL,
  `e` BIGINT unsigned NOT NULL,

  `f` TINYINT NOT NULL,
  `g` SMALLINT NOT NULL,
  `h` MEDIUMINT NOT NULL,
  `i` INT NOT NULL,
  `j` BIGINT NOT NULL,
  PRIMARY KEY (`a`)
) ENGINE=InnoDB;

INSERT INTO `test_int_max` VALUES (255, 65535, 16777215, 4294967295, 18446744073709551615, -1, -1, -1, -1, -1);
