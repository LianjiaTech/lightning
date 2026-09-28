CREATE DATABASE IF NOT EXISTS test;
USE test;

CREATE TABLE `t_compress` (
  `id` int NOT NULL,
  `v` varchar(50) DEFAULT NULL,
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
