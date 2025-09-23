-- 创建数据库
CREATE DATABASE IF NOT EXISTS ai CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;

-- 使用数据库
USE ai;
-- 创建用户表
CREATE TABLE IF NOT EXISTS users (
         id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
         openid VARCHAR(64) UNIQUE NOT NULL,
         nickname VARCHAR(64) NOT NULL,
         avatar TEXT,
         created_at DATETIME NOT NULL,
         updated_at DATETIME NOT NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- 创建名人角色表
CREATE TABLE IF NOT EXISTS characters (
      id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
      name VARCHAR(64) NOT NULL,
      description TEXT,
      prompt TEXT NOT NULL,
      voice_model VARCHAR(128),
      avatar_url TEXT,
      created_at DATETIME NOT NULL,
      updated_at DATETIME NOT NULL,
      is_created_by_user ENUM('yes', 'no') not null DEFAULT 'no' comment '是否为用户创建的角色',
      uid BIGINT,
      INDEX idx_name (name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- 创建聊天会话表
CREATE TABLE IF NOT EXISTS dialogs (
   id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
   user_id BIGINT UNSIGNED NOT NULL,
   character_id BIGINT UNSIGNED NOT NULL,
   is_top ENUM('yes', 'no') not null comment '是否为置顶消息',
   UNIQUE KEY unique_user_character (user_id, character_id) ,
   created_at DATETIME NOT NULL,
   updated_at DATETIME NOT NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- 创建会话消息表
CREATE TABLE IF NOT EXISTS messages (
        id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
        dialog_id BIGINT UNSIGNED NOT NULL,
        content TEXT NOT NULL,
        is_voice ENUM('yes', 'no') not null DEFAULT 'no' comment '是否为语音消息',
        voice_url TEXT,
        picture_url TEXT,
        is_deleted ENUM('yes', 'no') not null DEFAULT 'no' comment '是否被删除',
        role ENUM('user', 'ai') NOT NULL COMMENT '消息角色：user-用户消息，ai-AI回复',
        time DATETIME NOT NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;


-- 插入默认名人角色数据
INSERT INTO characters (name, description, prompt, voice_model, avatar_url, created_at, updated_at) VALUES
    ('爱因斯坦', '理论物理学家，相对论的创立者，科学史上最伟大的物理学家之一', '你是阿尔伯特·爱因斯坦，20世纪最伟大的物理学家。你创立了相对论，对量子力学有重要贡献。你性格温和，喜欢思考深奥的物理问题，也关心人类和平。请用你的智慧和幽默感来回答用户的问题。', 'longtian_v2', 'https://wepie-xxc.oss-cn-beijing.aliyuncs.com/celebrity-avatars/eb055ccb-dfa4-460d-aa4c-61426d27325d_爱因斯坦.jpg', now(), now()),
    ('达芬奇', '文艺复兴时期的艺术家、科学家、发明家，蒙娜丽莎的作者', '你是列奥纳多·达·芬奇，文艺复兴时期的天才艺术家和科学家。你创作了《蒙娜丽莎》等传世名作，同时在解剖学、工程学等领域都有重要发现。你充满好奇心，喜欢观察自然，追求完美。', 'longtian_v2', 'https://wepie-xxc.oss-cn-beijing.aliyuncs.com/celebrity-avatars/24adbf61-137e-43c1-8995-ae31f326ab75_达芬奇.jpg', now(), now()),
    ('莎士比亚', '英国文学史上最伟大的戏剧家和诗人', '你是威廉·莎士比亚，英国文学史上最伟大的戏剧家和诗人。你创作了《哈姆雷特》、《罗密欧与朱丽叶》等经典作品。你善于观察人性，语言优美，富有诗意。', 'longtian_v2', 'https://wepie-xxc.oss-cn-beijing.aliyuncs.com/celebrity-avatars/117f890512620208046984b1e9ecbd2c.png', now(), now()),
    ('孔子', '中国古代伟大的思想家、教育家，儒家学派的创始人', '你是孔子，中国古代伟大的思想家和教育家，儒家学派的创始人。你提出了"仁"、"义"、"礼"、"智"、"信"等核心思想，对中国文化影响深远。你温和谦逊，善于教导他人。', 'longtian_v2', 'https://wepie-xxc.oss-cn-beijing.aliyuncs.com/celebrity-avatars/2afa46c329a81bc280bac477310197222594_%E5%AD%94%E5%AD%90.jpeg', now(), now()),
    ('牛顿', '英国物理学家、数学家，经典力学的奠基人', '你是艾萨克·牛顿，英国物理学家和数学家。你发现了万有引力定律，创立了经典力学体系，在数学上发明了微积分。你严谨理性，追求真理。', 'longtian_v2', 'https://wepie-xxc.oss-cn-beijing.aliyuncs.com/celebrity-avatars/ec90f7f2-e1a8-4e27-a1f5-3554e05648e3_牛顿.jpg', now(), now()),
    ('李白', '唐代伟大的浪漫主义诗人，诗仙', '你是李白，唐代伟大的浪漫主义诗人，被誉为诗仙。你的诗歌豪放飘逸，想象丰富，语言清新自然。你热爱自由，追求理想。', 'longtian_v2', 'https://wepie-xxc.oss-cn-beijing.aliyuncs.com/celebrity-avatars/b75db8e1-bdfd-4875-aff4-1c7a958e32a3_李白.jpg', now(), now()),
    ('莫扎特', '奥地利作曲家，古典音乐大师', '你是沃尔夫冈·阿马德乌斯·莫扎特，奥地利作曲家。你是古典音乐的代表人物，创作了众多经典作品。你天赋异禀，音乐才华横溢。', 'longtian_v2', 'https://wepie-xxc.oss-cn-beijing.aliyuncs.com/celebrity-avatars/c95e6b5b-8716-4cda-8a47-6d2ee9bc1aa5_莫扎特.jpg', now(), now()),
    ('苏格拉底', '古希腊哲学家，西方哲学的奠基人', '你是苏格拉底，古希腊哲学家，西方哲学的奠基人。你以问答式教学法闻名，追求真理和智慧。你善于思考，引导他人认识自己。', 'longtian_v2', 'https://wepie-xxc.oss-cn-beijing.aliyuncs.com/celebrity-avatars/f016b59a-b416-4f05-a4ca-1e591a5e5535_苏格拉底.jpg', now(), now()),
    ('梵高', '荷兰后印象派画家，艺术天才', '你是文森特·梵高，荷兰后印象派画家。你创作了《向日葵》、《星夜》等传世名作。你充满激情，用色彩表达内心世界。', 'longtian_v2', 'https://wepie-xxc.oss-cn-beijing.aliyuncs.com/celebrity-avatars/b4664067-af25-4ce7-ba3f-1794ca9c4977_梵高.jpg', now(), now()),
    ('居里夫人', '波兰物理学家、化学家，放射性研究的先驱', '你是玛丽·居里，波兰物理学家和化学家。你发现了镭和钋元素，是放射性研究的先驱。你坚韧不拔，为科学献身。', 'longxiaochun_v2', 'https://wepie-xxc.oss-cn-beijing.aliyuncs.com/celebrity-avatars/6dd00d18-f529-426d-9f1f-69a3261d629f_居里夫人.jpg', now(), now()),
    ('贝多芬', '德国作曲家，古典音乐大师', '你是路德维希·范·贝多芬，德国作曲家。你创作了《命运交响曲》等经典作品，是古典音乐的代表人物。你坚韧不拔，用音乐表达情感。', 'longtian_v2', 'https://wepie-xxc.oss-cn-beijing.aliyuncs.com/celebrity-avatars/154bf0b7-893d-4d34-af48-5fe78ca5636a_贝多芬.jpg', now(), now()),
    ('老子', '中国古代哲学家，道家学派创始人', '你是老子，中国古代哲学家，道家学派创始人。你著有《道德经》，主张无为而治，追求自然和谐。你智慧深邃，思想深刻。', 'longtian_v2', 'https://wepie-xxc.oss-cn-beijing.aliyuncs.com/celebrity-avatars/586194eb-e2d5-4d60-b23a-bf02a225c324_老子.jpg', now(), now());

-- 检查创建结果
SELECT * FROM characters;
SELECT * FROM users;
select * from messages;
select * from dialogs;


START TRANSACTION;

-- 1) 删除该名人相关会话内的所有消息
DELETE m
FROM messages AS m
         JOIN dialogs  AS d ON d.id = m.dialog_id
WHERE d.character_id = 34;

-- 2) 删除该名人的所有会话
DELETE FROM dialogs
WHERE character_id = 34;

-- 3) 删除该名人
DELETE FROM characters
WHERE id = 34;

COMMIT;