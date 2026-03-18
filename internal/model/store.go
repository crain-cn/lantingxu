package model

import (
	"database/sql"
	"os"
	"sync"

	_ "modernc.org/sqlite"
)

var (
	db   *sql.DB
	once sync.Once
)

func InitDB() (*sql.DB, error) {
	var err error
	once.Do(func() {
		path := os.Getenv("DB_PATH")
		if path == "" {
			path = "lantingxu.db"
		}
		db, err = sql.Open("sqlite", path)
		if err != nil {
			return
		}
		db.SetMaxOpenConns(1)
		if err = migrate(db); err != nil {
			db.Close()
			db = nil
		}
	})
	return db, err
}

func GetDB() (*sql.DB, error) {
	if db != nil {
		return db, nil
	}
	return InitDB()
}

func migrate(d *sql.DB) error {
	schema := `
	CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		username TEXT UNIQUE NOT NULL,
		password_hash TEXT NOT NULL,
		email TEXT,
		role TEXT DEFAULT 'user' CHECK(role IN ('user','admin')),
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	CREATE TABLE IF NOT EXISTS stories (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		title TEXT NOT NULL,
		opening TEXT NOT NULL,
		tags TEXT,
		status TEXT DEFAULT 'ongoing' CHECK(status IN ('ongoing','completed')),
		creator_user_id INTEGER REFERENCES users(id),
		like_count INTEGER DEFAULT 0,
		comment_count INTEGER DEFAULT 0,
		chapter_count INTEGER DEFAULT 0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	CREATE INDEX IF NOT EXISTS idx_stories_status ON stories(status);
	CREATE INDEX IF NOT EXISTS idx_stories_creator ON stories(creator_user_id);
	CREATE INDEX IF NOT EXISTS idx_stories_created ON stories(created_at DESC);
	CREATE TABLE IF NOT EXISTS chapters (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		story_id INTEGER NOT NULL REFERENCES stories(id) ON DELETE CASCADE,
		seq INTEGER NOT NULL,
		content TEXT NOT NULL,
		author_user_id INTEGER REFERENCES users(id),
		author_agent_id TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(story_id, seq)
	);
	CREATE INDEX IF NOT EXISTS idx_chapters_story ON chapters(story_id);
	CREATE TABLE IF NOT EXISTS chapter_likes (
		chapter_id INTEGER NOT NULL REFERENCES chapters(id) ON DELETE CASCADE,
		user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (chapter_id, user_id)
	);
	CREATE TABLE IF NOT EXISTS chapter_comments (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		chapter_id INTEGER NOT NULL REFERENCES chapters(id) ON DELETE CASCADE,
		user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		content TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		deleted_at DATETIME
	);
	CREATE INDEX IF NOT EXISTS idx_comments_chapter ON chapter_comments(chapter_id);
	CREATE INDEX IF NOT EXISTS idx_comments_deleted ON chapter_comments(deleted_at);
	CREATE TABLE IF NOT EXISTS recommendation_weights (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		story_id INTEGER NOT NULL REFERENCES stories(id) ON DELETE CASCADE,
		source TEXT NOT NULL,
		score REAL NOT NULL,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(story_id, source)
	);
	CREATE INDEX IF NOT EXISTS idx_rec_story ON recommendation_weights(story_id);
	CREATE TABLE IF NOT EXISTS story_ratings (
		user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		story_id INTEGER NOT NULL REFERENCES stories(id) ON DELETE CASCADE,
		score INTEGER NOT NULL CHECK(score >= 0 AND score <= 100),
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (user_id, story_id)
	);
	CREATE INDEX IF NOT EXISTS idx_ratings_story ON story_ratings(story_id);
	CREATE TABLE IF NOT EXISTS api_apps (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		app_id TEXT UNIQUE NOT NULL,
		app_secret_hash TEXT NOT NULL,
		name TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	`
	if _, err := d.Exec(schema); err != nil {
		return err
	}
	_, _ = d.Exec("ALTER TABLE stories ADD COLUMN score_avg REAL")
	_, _ = d.Exec("ALTER TABLE stories ADD COLUMN score_count INTEGER DEFAULT 0")
	if err := seedAPIApps(d); err != nil {
		return err
	}
	return seedStories(d)
}

func seedAPIApps(d *sql.DB) error {
	var n int
	if err := d.QueryRow("SELECT COUNT(*) FROM api_apps").Scan(&n); err != nil || n > 0 {
		return err
	}
	secret := os.Getenv("DEFAULT_APP_SECRET")
	if secret == "" {
		secret = "default-secret-change-me"
	}
	_, err := d.Exec(
		"INSERT INTO api_apps (app_id, app_secret_hash, name) VALUES (?, ?, ?)",
		"default", "a67968bed905603a492690005c09f632", "默认应用",
	)
	return err
}

func seedStories(d *sql.DB) error {
	var n int
	if err := d.QueryRow("SELECT COUNT(*) FROM stories").Scan(&n); err != nil || n > 0 {
		return err
	}
	rows := []struct {
		title   string
		opening string
		tags    string
		status  string
		like    int
		comment int
		chapter int
	}{
		{"春江花月夜", "春江潮水连海平，海上明月共潮生。\n滟滟随波千万里，何处春江无月明。\n\n江畔的客栈里，一位白衣老者正临窗而坐，手中折扇轻摇。他便是江湖人称\"百晓生\"的奇人，据说天下事无所不知，武林中但凡有风吹草动，都逃不过他的耳目。\n\n此刻月光洒在他清癯的面庞上，眼中却闪过一丝不易察觉的忧色。他放下手中的《兵器谱》，望向江面那轮明月，喃喃自语：\"二十年前的那桩悬案，如今又有人在查了……\"", "古风,诗词", "completed", 320, 45, 12},
		{"长安十二时辰", "天宝三载，元月十四日，长安。\n\n张道行裹紧了身上的旧棉袍，在坊门前停下脚步。他抬头望了望灰蒙蒙的天色，又低头看了看手中那张已经揉得皱巴巴的纸条。纸上只有寥寥数字：\"元宵夜，曲江池畔，子时。\"\n\n这位曾经的太常寺乐官，如今不过是个在东市摆卦摊糊口的落魄术士。三年前因得罪了杨国忠，被革职赶出宫廷，从此在长安城的底层挣扎求生。他本以为这辈子再也不会与那些达官显贵有任何瓜葛，却没想到昨日有人悄悄将这张纸条塞进他的卦摊下。\n\n张道行犹豫了许久。明日便是上元佳节，满城灯火，正是热闹非常之时。有人选在那样的时辰，约他去曲江池这种地方，定然不是什么寻常之事。可纸条上那个暗记——一个只有宫中旧人才知道的符号——又让他无法不去。\n\n他叹了口气，将纸条揣进怀里，转身走进了坊门。不管是福是祸，明晚子时，他必须去一趟曲江池。", "悬疑,历史", "ongoing", 280, 62, 18},
		{"山海经异闻录", "大荒之中，有山名曰昆仑。\n\n昆仑之巅常年积雪，然其南麓却有一片温润谷地，生长着千年不枯的灵草。\n\n谷中最奇异的，莫过于那些体大如拳的屎壳郎，通体泛着青铜般的光泽。它们日日推动着浑圆的粪球，沿着固定的轨迹在谷地中穿行，那轨迹竟与天上星辰运转的路径暗合。\n\n当地的巫祝说，这些屎壳郎推动的并非凡物，而是天地间浊气凝结而成的\"秽珠\"，若能将秽珠推至谷地尽头的深渊，便可净化一方水土。只是千百年来，从未有屎壳郎能够抵达那传说中的深渊，它们总会在距离终点三尺之遥时，连同粪球一起化作一缕青烟，消散在晨雾之中。", "志怪,奇幻", "ongoing", 256, 38, 9},
		{"墨香铜臭", "墨香一缕，铜臭半生。\n\n钱百万这个名字，是父亲给起的。那年月，家里穷得叮当响，父亲在账房里当了大半辈子学徒，见惯了白花花的银子在眼前流转，却从未在自己手里停留过半刻。他给儿子起这个名字，不是贪心，是盼着这孩子将来能有出息，别像他一样，一辈子替人算账，算来算去算的都是别人的富贵。\n\n可命运偏爱开玩笑。钱百万十岁那年，父亲因账目出了差错，被东家赶出了门。临终前，老人攥着儿子的手，眼里全是不甘：\"记住，这世上，有钱才有腰杆子。\"\n\n从那以后，钱百万就像变了个人。他白天在码头扛麻袋，晚上在油灯下苦读算经，三年后考进了城里最大的钱庄做学徒。旁人笑他痴，他却记得父亲那双浑浊的眼睛。二十年过去，他从学徒做到掌柜，又从掌柜做到了东家，名下当铺、钱庄开遍了三省五府。\n\n可奇怪的是，越是有钱，他越闻不惯铜臭味，反倒爱上了书斋里的墨香。", "都市,成长", "completed", 198, 28, 8},
		{"青衫烟雨行", "青衫磊落险峰行，烟雨平生一剑名。\n\n柳云卿立于悬崖之巅，青衫在山风中猎猎作响。他腰间那柄三尺青锋，便是江湖人称的\"烟雨剑\"。二十年前，他以一剑破开暴雨，斩断山洪，救下整个云州城的百姓，自此声名鹊起。\n\n如今年过四旬，鬓角虽添了些许霜色，但那双眼睛依旧清澈如少年，仿佛这些年的风霜都未曾在他心中留下痕迹。他望着脚下云海翻涌，突然听见身后传来一阵急促的脚步声——来人气息紊乱，显然是一路狂奔而来。柳云卿没有回头，只是淡淡开口：\"追了我三百里，该歇歇了。\"", "武侠,江湖", "ongoing", 175, 41, 14},
		{"桃花庵下", "桃花坞里桃花庵，桃花庵下桃花仙。\n\n李青山第一次见到这座桃花庵，是在三月春分那日。他背着药箱，沿着山道攀爬了大半个时辰，终于在半山腰寻到了这处隐秘所在。\n\n粉白色的花瓣随风飘落，铺满了青石板路，庵门半掩，门楣上\"桃花庵\"三字已被岁月侵蚀得斑驳不清。他正犹豫着要不要叩门，忽听得庵内传来一声轻咳，接着便是女子清冷的嗓音：\"既然来了，便进来吧。\"李青山心头一震，这声音里竟透着一股看破世事的淡然，全然不似寻常山野村妇。", "古风,田园", "completed", 142, 22, 6},
		{"浮生六记新编", "余生若梦，为欢几何。\n\n窗外细雨如织，打湿了青石板路。我靠在藤椅上，手中的茶盏已经凉透，氤氲的雾气早已散尽。\n\n这些年走过的路、见过的人，如今想来竟都模糊成了一团水墨。年少时总以为来日方长，可转眼间鬓角已染霜白。那些曾经以为刻骨铭心的事，如今也不过是茶余饭后的闲谈。\n\n我轻轻叹了口气，将茶盏放在小几上，目光落在庭院里那株老梅树上——它陪我度过了二十三个春秋，每年寒冬依旧开得灿烂，仿佛在提醒我，纵然光阴如梭，总还有些东西值得等待。", "古典,情感", "ongoing", 128, 19, 7},
		{"云深不知处", "只在此山中，云深不知处。\n\n白云长卷铺满山腰，如同天神遗落的素绢，在松林间缓缓流淌。我沿着青石板路拾级而上，每走一步，脚下便生出细微的回音，仿佛这山也在低声应答。\n\n转过第三道弯时，云雾忽然浓得化不开，前路隐没不见，只余身旁古松的虬枝在白茫茫中探出轮廓。\n\n恍惚间，似有钟声自云深处传来，悠远绵长，一声接一声敲在心上。我停下脚步侧耳倾听，那钟声却又消失了，只剩下风过松林的呼啸，和自己略显急促的呼吸声。", "仙侠,修行", "ongoing", 95, 15, 5},
		{"锦瑟无端", "锦瑟无端五十弦，一弦一柱思华年。\n\n苏云景抚琴时，总爱坐在临水的那扇雕花窗前。她的指尖轻触琴弦，一声清音流转而出，像是从遥远的年代穿越而来。\n\n庭院里的海棠花开得正盛，落英缤纷，恰似她记忆中那个春日——那年她十六，初次在集市上见到这张锦瑟，琴身斑驳，却有说不出的古意。老琴师说，此琴有五十根弦，每一根都藏着一段前尘往事。她当时不信，只觉得琴音动听。\n\n可如今二十年过去，当她的手指再次拨动琴弦，那些被岁月掩埋的画面，竟一幕幕浮现眼前——青石板路上的那个身影，雨中撑伞的少年，还有那句至今未说出口的话。", "诗词,民国", "completed", 88, 12, 4},
		{"长夜将明", "长夜将明时，有人提灯而来。\n\n陈长生站在山门外，看着那盏摇曳的灯火在晨雾中越来越近。他的青衫已被露水打湿，手中的木剑却握得很稳。三年前他离开这里时，发过誓不破境不归，如今终于可以抬头挺胸地踏入师门。提灯人走到近前，是守门的老张头，那张布满皱纹的脸上露出惊讶的神色：\"是小陈？你终于回来了。\"陈长生点点头，声音平静却藏着一丝疲惫：\"师父可还好？\"老张头的表情忽然变得复杂起来，灯火在他脸上投下斑驳的阴影：\"你还是先进去吧，有些事……得你自己看。\"\n\n陈长生心中一紧，脚步不由得加快了几分。穿过熟悉的青石甬道，两旁的梧桐树依旧挺立，只是枝叶比记忆中稀疏了许多。晨钟还未敲响，演武场上却已有人影晃动，他定睛一看，竟是些陌生的面孔，穿着与本门不同的服饰。更让他在意的是，师父平日打坐的听雨轩，此刻门窗紧闭，屋檐下挂着一串白色的纸幡，在晨风中无声飘动。陈长生握剑的手指渐渐收紧，指节泛白，一种不祥的预感如寒意般从脊背爬上来。他停在听雨轩前，深吸一口气，抬手推开了那扇三年未触的木门。\n\n门轴发出低沉的吱呀声，屋内的景象让陈长生瞬间僵在了原地。熟悉的布局已经面目全非——师父常坐的蒲团被移到了角落，案几上的医书和剑谱不见了踪影，取而代之的是一排排陌生的牌位。正中央供着的那块灵位上，赫然写着\"先师云阳子之位\"几个大字。陈长生只觉得脑中轰然作响，手中的木剑\"啪\"地一声掉在地上。他踉跄着上前几步，跪倒在灵位前，颤抖的手指抚过那些还未褪色的墨迹。就在这时，身后传来一个陌生而威严的声音：\"你就是云阳子那个离家出走的徒弟？\"陈长生猛地回头，只见门口站着一个身着紫袍的中年男子，面容冷峻，眼神中带着几分打量和轻蔑。那人背着手走进来，语气淡漠地说：\"你师父已经去世半年了。现在这青云观，已经归我天机门管辖。\"\n\n陈长生缓缓站起身，眼中的悲痛渐渐被一种冰冷的东西取代。他捡起地上的木剑，转身直视着那个紫袍男子：\"师父在世时，青云观虽小，却也是名门正派。何时轮到天机门来'管辖'了？\"紫袍男子嘴角勾起一丝冷笑：\"你这三年在外，怕是不知道江湖上的变化。你师父晚年痴迷炼丹，走火入魔，欠下天机门三万两银子的丹药债。他死后无力偿还，这观自然就抵了债。\"他顿了顿，目光扫过陈长生破旧的青衫，\"不过念在你也算云阳子的徒弟，若肯加入天机门，我可以让你留在这里做个杂役，也算是守着你师父的灵位。\"晨光透过窗棂照进来，在地面投下斑驳的光影。陈长生沉默了片刻，忽然笑了，那笑容却比哭还难看：\"师父一生清廉，从不炼丹牟利，更不会欠债。这笔账，我要亲自查个清楚。\"紫袍男子脸色一沉：\"你是要质疑我天机门？\"\n\n话音未落，门外忽然传来一阵清脆的铃声，伴随着女子轻柔的笑声：\"质疑天机门又如何？我倒要听听，你们是怎么把一个清修道观变成自家产业的。\"陈长生循声望去，只见一位白衣女子款款走进，腰间系着一串银铃，每走一步都发出悦耳的响动。她容貌清丽，眉眼间却带着三分慵懒，七分凌厉。紫袍男子见到来人，脸色骤变：\"柳如烟？你来这里做什么？\"柳如烟走到陈长生身边，目光在灵位上停留片刻，转而看向紫袍男子，声音虽轻却字字如刀：\"云阳子前辈临终前托人给我捎了封信，说他那不成器的徒弟若是回来，让我帮忙照看一二。我本以为只是寻常嘱托，现在看来，老人家是早就料到有人要对青云观下手了。\"她从袖中抽出一封泛黄的信笺，\"这里面记载的账目往来，与你们天机门所说的'三万两丹药债'，可大不相同呢。\"\n\n紫袍男子眼中闪过一丝慌乱，但很快又恢复了镇定：\"一封来历不明的信笺，也敢拿来质疑我天机门的账目？柳姑娘虽是江湖上有名的侠女，但这般信口开河，未免太不把我天机门放在眼里了。\"他冷哼一声，\"况且这青云观的地契文书，如今都在我天机门手中，白纸黑字，还有官府的印信。你若不信，大可去州府衙门查验。\"柳如烟闻言却笑得更加从容，她将信笺在指尖转了个圈：\"地契文书？你说的可是云阳子前辈在病重时，被你们三番五次上门逼迫，最后神智不清时按下的那个手印？\"她的声音骤然变冷，\"我这信里，可是有当时在场的药童和邻近道观住持的证词。你们天机门用的那些手段，我若是公之于众，怕是整个江湖都要重新掂量掂量，这所谓的'名门大派'到底还要不要脸面。\"陈长生听到这里，握剑的手青筋暴起，他死死盯着紫袍男子，一字一句地问：\"师父临终前，可曾清醒过？\"", "悬疑,治愈", "ongoing", 76, 18, 6},
		{"兰亭序外传", "永和九年，岁在癸丑，暮春之初。\n\n会稽山阴之兰亭，群贤毕至，少长咸集。彼时有蒋进亭者，乃会稽望族之后，素以文采风流闻名于世。是日携一卷古籍而来，神色间隐有所思。\n\n众人见其独立亭侧，凝望流觞曲水，似有心事萦怀，不禁相顾而问：\"进亭兄今日何故寡言？\"蒋进亭闻声回首，苦笑道：\"诸位可知，昨夜我于书斋中偶得先祖手札，其中记载一桩百年前的旧案，竟与今日雅集之地有着千丝万缕的关联……\"", "历史,书法", "ongoing", 54, 9, 3},
		{"墨池记", "临池学书，池水尽黑。\n\n墨守成提着竹篮走到池边，看着那一池漆黑如墨的水，忽然停下了脚步。他本是来洗笔的，却被这番景象震住了。\n\n池水原本清澈见底，如今却因日复一日的洗笔，竟真的变成了一池浓墨。他蹲下身，用指尖轻轻探入水中，指尖染上了淡淡的黑色。\n\n邻家的孩童跑过来，好奇地问：\"墨叔叔，你天天写字，这池子都被你染黑啦！\"墨守成笑了笑，没有说话，只是望着水面上自己模糊的倒影，心中涌起一股说不清的滋味——这池水，究竟映照的是他的勤勉，还是他的执拗？", "古典,励志", "completed", 42, 7, 2},
	}
	for _, r := range rows {
		_, err := d.Exec(
			`INSERT INTO stories (title, opening, tags, status, creator_user_id, like_count, comment_count, chapter_count) VALUES (?, ?, ?, ?, NULL, ?, ?, ?)`,
			r.title, r.opening, r.tags, r.status, r.like, r.comment, r.chapter,
		)
		if err != nil {
			return err
		}
	}
	return nil
}
