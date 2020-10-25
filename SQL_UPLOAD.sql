-- Delegao.moe initial SQL dump
-- Created on 13 Sep. 2020
--


SET SQL_MODE = "NO_AUTO_VALUE_ON_ZERO";
SET AUTOCOMMIT = 0;
START TRANSACTION;
SET time_zone = "+00:00";


/*!40101 SET @OLD_CHARACTER_SET_CLIENT=@@CHARACTER_SET_CLIENT */;
/*!40101 SET @OLD_CHARACTER_SET_RESULTS=@@CHARACTER_SET_RESULTS */;
/*!40101 SET @OLD_COLLATION_CONNECTION=@@COLLATION_CONNECTION */;
/*!40101 SET NAMES utf8mb4 */;


--
--  `guestbook` table structure
--

CREATE TABLE `guestbook` (
  `id` int(7) NOT NULL,
  `name` text COLLATE utf8mb4_spanish_ci NOT NULL,
  `email` varchar(128) COLLATE utf8mb4_spanish_ci NOT NULL,
  `comment` varchar(512) COLLATE utf8mb4_spanish_ci NOT NULL,
  `country` text COLLATE utf8mb4_spanish_ci NOT NULL,
  `date` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_spanish_ci;

-- --------------------------------------------------------

--
-- `news` table structure
--

CREATE TABLE `news` (
  `id` int(7) NOT NULL,
  `title` varchar(255) COLLATE utf8mb4_spanish_ci NOT NULL,
  `body` mediumtext COLLATE utf8mb4_spanish_ci NOT NULL,
  `status` int(1) NOT NULL DEFAULT '0',
  `tags` varchar(1000) COLLATE utf8mb4_spanish_ci NOT NULL,
  `posted_by` varchar(100) COLLATE utf8mb4_spanish_ci NOT NULL,
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `last_update` datetime DEFAULT NULL ON UPDATE CURRENT_TIMESTAMP
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_spanish_ci;

-- --------------------------------------------------------

--
-- `users` table structure
--

CREATE TABLE `users` (
  `id` int(11) NOT NULL,
  `username` varchar(50) COLLATE utf8mb4_spanish_ci NOT NULL,
  `password` varchar(255) COLLATE utf8mb4_spanish_ci NOT NULL,
  `created_at` datetime DEFAULT CURRENT_TIMESTAMP
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_spanish_ci;


--
-- Dumped tables index
--

--
-- `guestbook` table index
--
ALTER TABLE `guestbook`
  ADD PRIMARY KEY (`id`);

--
-- `news` table index
--
ALTER TABLE `news`
  ADD PRIMARY KEY (`id`);

--
-- `users` table index
--
ALTER TABLE `users`
  ADD PRIMARY KEY (`id`),
  ADD UNIQUE KEY `username` (`username`);

/*!40101 SET CHARACTER_SET_CLIENT=@OLD_CHARACTER_SET_CLIENT */;
/*!40101 SET CHARACTER_SET_RESULTS=@OLD_CHARACTER_SET_RESULTS */;
/*!40101 SET COLLATION_CONNECTION=@OLD_COLLATION_CONNECTION */;
