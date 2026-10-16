# MovieHub — ใบงานสัปดาห์ที่ 12

Go API และ PostgreSQL ตาม [ใบงานช่องขายตั๋วกับพนักงานตรวจตั๋ว](https://plearnjai.com/be2026/Lab-Week12-Ticket-Booth-and-Gatekeepers.html) รองรับทั้ง 7 ด่าน รวมรายการอยากดูและงานต่อยอดการให้คะแนน

## เตรียมฐานข้อมูล

สั่งจากราก repository:

```bash
docker compose up -d db
docker compose exec -T db createdb -U postgres moviehub
docker compose exec -T db psql -U postgres -d moviehub -v ON_ERROR_STOP=1 < moviehub/schema.sql
docker compose exec -T db psql -U postgres -d moviehub -v ON_ERROR_STOP=1 < moviehub/seed.sql
```

คำสั่งสร้างฐานข้อมูลและโหลด SQL ใช้สำหรับฐานข้อมูลใหม่เท่านั้น ถ้ามี `moviehub` และข้อมูลอยู่แล้ว ให้ใช้ฐานข้อมูลเดิม

## เปิด API

```bash
cd moviehub
cp -n .env.example .env
# สร้างกุญแจครั้งแรกเท่านั้น อย่าทับกุญแจเดิม
test -f .jwt_secret || openssl rand -hex 32 > .jwt_secret
go run .
```

เซิร์ฟเวอร์อ่าน `.env` ให้อัตโนมัติ และอ่านกุญแจจาก `JWT_SECRET` หรือไฟล์ `.jwt_secret` กุญแจต้องยาวอย่างน้อย 32 ตัวอักษร ทั้งสองไฟล์ถูกละเว้นโดย git แล้ว ตั๋วใช้ HS256 และมีอายุหนึ่งชั่วโมง

## เส้นทาง API

| Method | เส้นทาง | สิทธิ์ | ผลสำเร็จ |
| --- | --- | --- | --- |
| POST | `/api/auth/register` | สาธารณะ | 201 ข้อมูลสมาชิก |
| POST | `/api/auth/login` | สาธารณะ | 200 `{token, member}` |
| GET | `/api/movies?q=evil` | สาธารณะ | 200 `{items: [...]}` |
| GET | `/api/movies/{id}` | สาธารณะ | 200 ข้อมูลหนังและเรื่องย่อ |
| GET | `/api/movies/{id}/reviews` | สาธารณะ | 200 `{items: [...]}` |
| GET | `/api/me` | เข้าสู่ระบบ | 200 ข้อมูลเจ้าของตั๋ว |
| POST | `/api/movies/{id}/reviews` | เข้าสู่ระบบ | 201 `{id, text, createdAt}` |
| DELETE | `/api/reviews/{reviewId}` | เจ้าของรีวิวหรือแอดมิน | 204 |
| GET | `/api/admin/members` | แอดมิน | 200 `{items: [...]}` |
| GET | `/api/me/wishlist` | เข้าสู่ระบบ | 200 `{items: [...]}` |
| PUT | `/api/me/wishlist/{movieId}` | เข้าสู่ระบบ | 200 `{movieId, note, addedAt}` |
| DELETE | `/api/me/wishlist/{movieId}` | เข้าสู่ระบบ | 204 |
| PUT | `/api/movies/{id}/vote` | เข้าสู่ระบบ | 200 `{movieId, score}` |
| DELETE | `/api/movies/{id}/vote` | เข้าสู่ระบบ | 204 |

เส้นทางที่ต้องเข้าสู่ระบบใช้ `Authorization: Bearer <TOKEN>` เลขสมาชิกอ้างอิงจากตั๋วเสมอ การส่ง `memberId` หรือ `role` มาใน JSON ไม่ได้เพิ่มสิทธิ์

สมัครด้วย `{email, password, displayName}` รหัสผ่านต้องมีอย่างน้อย 8 ตัวอักษรและไม่เกิน 72 ไบต์ ชื่อไม่เกิน 50 ตัวอักษร การสมัครได้บทบาท `member` เสมอ และ API ไม่ส่ง password hash กลับมา

รีวิวรับ `{"text":"ข้อความอย่างน้อยสิบตัวอักษร"}` สมาชิกรีวิวหนังหนึ่งเรื่องได้ครั้งเดียว รีวิวซ้ำตอบ 409 หนังไม่มีจริงตอบ 404 และการลบรีวิวของคนอื่นตอบ 404

รายการอยากดูรับ `{"note":"ดูกับเพื่อน"}` หรือ body ว่าง โน้ตยาวได้ไม่เกิน 200 ตัวอักษร กด PUT ซ้ำจะอัปเดตโน้ตของรายการเดิมโดยคง `addedAt` เดิม แต่ละรายการใน GET มีรูปแบบ `{movieId, note, addedAt, movie: {id, title, titleTh, genre, year, rating, poster}}`

คะแนนรับ `{"score":8}` ต้องเป็นจำนวนเต็ม 1–10 ให้คะแนนใหม่จะอัปเดตคะแนนเดิม และ GET รีวิวจะแนบคะแนนของผู้เขียนเมื่อมีคะแนนอยู่ การลบคะแนนหรือรายการอยากดูซ้ำตอบ 204

ข้อผิดพลาดใช้รูปแบบ `{"error":{"code":"...","message":"..."}}` เมื่อเข้าสู่ระบบด้วยอีเมลที่ไม่มีอยู่ ระบบยังตรวจ bcrypt กับ hash จำลอง เพื่อให้เวลาตอบกลับใกล้เคียงกับกรณีรหัสผ่านผิด

## ทดลองใช้งาน

```bash
curl -i -X POST localhost:8080/api/auth/register \
  -H 'Content-Type: application/json' \
  -d '{"email":"nida@example.com","password":"popcorn2569","displayName":"นิดา"}'

curl -s -X POST localhost:8080/api/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"nida@example.com","password":"popcorn2569"}'

# นำ token จากคำตอบด้านบนมาใส่
export TOKEN='วางตั๋วตรงนี้'
curl -i localhost:8080/api/me -H "Authorization: Bearer $TOKEN"
curl -i -X POST localhost:8080/api/movies/1423191/reviews \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"text":"ตื่นเต้นตั้งแต่ฉากแรกจนจบ ซอมบี้ทำถึงมาก"}'
curl -i -X PUT localhost:8080/api/me/wishlist/1423191 \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"note":"ดูกับเพื่อน"}'
curl -i -X PUT localhost:8080/api/movies/1423191/vote \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"score":8}'

go run ./cmd/hashlab
go run ./cmd/peek "$TOKEN"
```

ตั้งแอดมินสำหรับทดลองจากราก repository หลังสมัครบัญชีนั้นแล้ว:

```bash
docker compose exec -T db psql -U postgres -d moviehub \
  -c "UPDATE members SET role = 'admin' WHERE email = 'staff@example.com';"
```

ต้องเข้าสู่ระบบใหม่เพื่อได้ตั๋วที่มีบทบาท `admin` ตั๋วที่ออกก่อนเปลี่ยนบทบาทยังมีบทบาทเดิมจนหมดอายุ

## เชื่อม React และตรวจ CORS

workspace นี้มีเฉพาะ backend ตั้ง `ALLOWED_ORIGINS=http://localhost:3000` ใน `.env` ของ Go หากมีหลาย origin ให้คั่นด้วยจุลภาค แล้วสร้าง `.env.local` ในโปรเจกต์ React:

```dotenv
REACT_APP_API_URL=http://localhost:8080
```

เริ่ม React ใหม่ แล้วปรับบรรทัดจัดการข้อผิดพลาดใน `apiFetch`:

```js
if (!res.ok) throw new Error(data.error?.message ?? data.error ?? `status ${res.status}`);
```

ทดสอบ preflight โดยไม่ต้องส่งตั๋ว:

```bash
curl -i -X OPTIONS localhost:8080/api/reviews/1 \
  -H 'Origin: http://localhost:3000' \
  -H 'Access-Control-Request-Method: DELETE' \
  -H 'Access-Control-Request-Headers: authorization'
```

จะได้ 204 พร้อม `Access-Control-Allow-Origin`, `Allow-Methods`, `Allow-Headers`, `Max-Age` และ `Vary: Origin` ถ้าใช้ origin นอกบัญชีอนุญาต จะได้ 204 โดยไม่มีหัวจดหมาย `Access-Control-*`

## ทดสอบ

จากโฟลเดอร์ `moviehub`:

```bash
go test ./...
go build ./...
go vet ./...
```

ชุดทดสอบปกติครอบคลุมตั๋วปลอมทั้ง 6 แบบ การตรวจ login, การปิดเส้นทางที่ต้องเข้าสู่ระบบ, การปฏิเสธสมาชิกทั่วไปจากหน้าแอดมิน และ CORS

หากต้องการทดสอบ API กับ PostgreSQL จริง ให้สร้างฐานข้อมูลทดสอบจากราก repository:

```bash
docker compose exec -T db createdb -U postgres moviehub_lab_test
cd moviehub
TEST_DATABASE_URL='postgres://postgres:secret@localhost:5432/moviehub_lab_test?sslmode=disable' \
  go test -count=1 -v ./...
```

ชุดทดสอบจะสร้าง schema ชั่วคราวและลบทิ้งเมื่อจบ ครอบคลุมการสมัคร/เข้าสู่ระบบ การป้องกันการสวมรอยเขียนรีวิว สิทธิ์ลบรีวิว ตั๋วเก่าและใหม่หลังเลื่อนบทบาท และการแยก wishlist/คะแนนระหว่างบัญชี หากไม่ตั้ง `TEST_DATABASE_URL` จะข้ามเฉพาะชุดทดสอบนี้
