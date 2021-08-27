package emotes

import (
	"fmt"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/SevenTV/ServerGo/src/aws"
	"github.com/SevenTV/ServerGo/src/configure"
	"github.com/SevenTV/ServerGo/src/discord"
	"github.com/SevenTV/ServerGo/src/mongo"
	"github.com/SevenTV/ServerGo/src/mongo/datastructure"
	"github.com/SevenTV/ServerGo/src/server/api/v2/rest/restutil"
	"github.com/SevenTV/ServerGo/src/server/middleware"
	"github.com/SevenTV/ServerGo/src/utils"
	"github.com/SevenTV/ServerGo/src/validation"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"gopkg.in/gographics/imagick.v3/imagick"
)

const MAX_FRAME_COUNT = 4096
const MAX_FILE_SIZE float32 = 2500000
const MAX_PIXEL_HEIGHT = 3000
const MAX_PIXEL_WIDTH = 3000

func CreateEmoteRoute(router fiber.Router) {

	rl := configure.Config.GetIntSlice("limits.route.emote-create")
	router.Post(
		"/",
		middleware.UserAuthMiddleware(true),
		middleware.RateLimitMiddleware("emote-create", int32(rl[0]), time.Millisecond*time.Duration(rl[1])),
		func(c *fiber.Ctx) error {
			c.Set("Content-Type", "application/json")
			usr, ok := c.Locals("user").(*datastructure.User)
			if !ok {
				return restutil.ErrLoginRequired().Send(c)
			}
			if !usr.HasPermission(datastructure.RolePermissionEmoteCreate) {
				return restutil.ErrAccessDenied().Send(c)
			}

			req := c.Request()
			fctx := c.Context()
			if !req.IsBodyStream() {
				return restutil.ErrBadRequest().Send(c, "Not A File Stream")
			}

			// Get file stream
			file := fctx.RequestBodyStream()
			mr := multipart.NewReader(file, utils.B2S(req.Header.MultipartFormBoundary()))
			var emote *datastructure.Emote
			var emoteName string              // The name of the emote
			var emoteTags []string            // The emote's tags, if any
			var channelID *primitive.ObjectID // The channel creating this emote
			var contentType string
			var ext string
			id, _ := uuid.NewRandom()

			// The temp directory where the emote will be created
			fileDir := fmt.Sprintf("%s/%s", configure.Config.GetString("temp_file_store"), id.String())
			if err := os.MkdirAll(fileDir, 0777); err != nil {
				log.WithError(err).Error("mkdir")
				return restutil.ErrInternalServer().Send(c)
			}
			ogFilePath := fmt.Sprintf("%v/og", fileDir) // The original file's path in temp

			// Remove temp dir once this function completes
			defer os.RemoveAll(fileDir)

			// Get form data parts
			channelID = &usr.ID // Default channel ID to the uploader
			for {
				part, err := mr.NextPart()
				if err == io.EOF {
					break
				} else if err != nil {
					log.WithError(err).Error("multipart_reader")
					break
				}

				switch part.FormName() {
				case "name":
					buf := make([]byte, 100)
					n, err := part.Read(buf)
					if err != nil && err != io.EOF {
						return restutil.ErrBadRequest().Send(c, "Emote Name Not Readable")
					}
					emoteName = utils.B2S(buf[:n])
				case "tags":
					b, err := io.ReadAll(part)
					if err != nil {
						return restutil.ErrBadRequest().Send(c, "Couldn't parse tags")
					}

					emoteTags = strings.Split(utils.B2S(b), ",")
					// Validate tags
					if len(emoteTags) > 6 {
						return restutil.ErrBadRequest().Send(c, "Too Many Tags (6)")
					}
					if ok, badTag := validation.ValidateEmoteTags(emoteTags); !ok {
						return restutil.ErrBadRequest().Send(c, fmt.Sprintf("'%s' is not a valid tag", badTag))
					}
				case "channel":
					buf := make([]byte, 64)
					n, err := part.Read(buf)
					if err != nil && err != io.EOF {
						return restutil.ErrBadRequest().Send(c, "User ID Not Readable")
					}
					id, err := primitive.ObjectIDFromHex(utils.B2S(buf[:n]))
					if err != nil {
						return restutil.ErrBadRequest().Send(c, "Invalid User ID")
					}
					channelID = &id
				case "emote":
					if emoteName == "" { // Infer emote name from file name if it wasn't specified
						basename := part.FileName()
						emoteName = strings.TrimSuffix(basename, filepath.Ext(basename))
					}

					data := make([]byte, chunkSize)
					contentType = part.Header.Get("Content-Type")
					switch contentType {
					case "image/jpeg":
						ext = "jpg"
					case "image/png":
						ext = "png"
					case "image/gif":
						ext = "gif"
					case "image/webp":
						ext = "webp"
					default:
						return restutil.ErrBadRequest().Send(c, "Unsupported File Type (want jpg, png, gif or webp)")
					}

					osFile, err := os.Create(ogFilePath)
					if err != nil {
						log.WithError(err).Error("file")
						return restutil.ErrInternalServer().Send(c)
					}

					byteSize := 0
					for {
						n, err := part.Read(data)
						byteSize += n
						if float32(byteSize) >= MAX_FILE_SIZE {
							return restutil.ErrBadRequest().Send(c, fmt.Sprintf("Input File Too Large. Must be <%vMB", MAX_FILE_SIZE/1000000))
						}

						if err != nil && err != io.EOF {
							log.WithError(err).Error("read")
							return restutil.ErrBadRequest().Send(c, "File Not Readable")
						}
						_, err2 := osFile.Write(data[:n])
						if err2 != nil {
							osFile.Close()
							log.WithError(err).Error("write")
							return restutil.ErrInternalServer().Send(c)
						}
						if err == io.EOF {
							break
						}
					}
				}
			}

			if emoteName == "" || channelID == nil {
				return restutil.ErrBadRequest().Send(c, "Uncomplete Form")
			}
			if !validation.ValidateEmoteName(utils.S2B(emoteName)) {
				return restutil.ErrBadRequest().Send(c, "Invalid Emote Name")
			}

			if !usr.HasPermission(datastructure.RolePermissionManageUsers) {
				if channelID.Hex() != usr.ID.Hex() {
					if err := mongo.Collection(mongo.CollectionNameUsers).FindOne(c.Context(), bson.M{
						"_id":     channelID,
						"editors": usr.ID,
					}).Err(); err != nil {
						if err == mongo.ErrNoDocuments {
							return restutil.ErrAccessDenied().Send(c)
						}
						log.WithError(err).Error("mongo")
						return restutil.ErrInternalServer().Send(c)
					}
				}
			}

			// Get uploaded image file into an image.Image
			ogFile, err := os.Open(ogFilePath)
			if err != nil {
				log.WithError(err).Error("could not open original file")
				return restutil.ErrInternalServer().Send(c)
			}
			ogHeight := 0
			ogWidth := 0
			switch ext {
			case "jpg":
				img, err := jpeg.Decode(ogFile)
				if err != nil {
					log.WithError(err).Error("could not decode jpeg")
					return restutil.ErrBadRequest().Send(c, fmt.Sprintf("Couldn't decode JPEG: %v", err.Error()))
				}
				ogWidth = img.Bounds().Dx()
				ogHeight = img.Bounds().Dy()
			case "png":
				img, err := png.Decode(ogFile)
				if err != nil {
					log.WithError(err).Error("could not decode png")
					return restutil.ErrBadRequest().Send(c, fmt.Sprintf("Couldn't decode PNG: %v", err.Error()))
				}
				ogWidth = img.Bounds().Dx()
				ogHeight = img.Bounds().Dy()
			case "gif":
				g, err := gif.DecodeAll(ogFile)
				if err != nil {
					log.WithError(err).Error("could not decode gif")
					return restutil.ErrBadRequest().Send(c, fmt.Sprintf("Couldn't decode GIF: %v", err.Error()))
				}

				// Set a cap on how many frames are allowed
				if len(g.Image) > MAX_FRAME_COUNT {
					return restutil.ErrBadRequest().Send(c, fmt.Sprintf("Maximum Frame Count Exceeded (%v)", MAX_FRAME_COUNT))
				}

				ogWidth, ogHeight = getGifDimensions(g)
			case "webp":
				return restutil.ErrBadRequest().Send(c, fmt.Sprintf("Sorry, direct support for WebP uploads is not available yet."))
			default:
				return restutil.ErrBadRequest().Send(c, "Unsupported File Format")
			}
			if ogWidth > MAX_PIXEL_WIDTH || ogHeight > MAX_PIXEL_HEIGHT {
				return restutil.ErrBadRequest().Send(c, fmt.Sprintf("Too Many Pixels (maximum %dx%d)", MAX_PIXEL_WIDTH, MAX_PIXEL_HEIGHT))
			}

			files := datastructure.EmoteUtil.GetFilesMeta(fileDir)
			mime := "image/webp"

			sizeX := [4]int16{0, 0, 0, 0}
			sizeY := [4]int16{0, 0, 0, 0}
			// Resize the frame(s)
			for i, file := range files {
				scope := file[1]
				sizes := strings.Split(file[2], "x")
				maxWidth, _ := strconv.ParseFloat(sizes[0], 4)
				maxHeight, _ := strconv.ParseFloat(sizes[1], 4)
				quality := file[3]
				outFile := fmt.Sprintf("%v/%v.webp", fileDir, scope)

				// Get calculed ratio for the size
				width, height := utils.GetSizeRatio(
					[]float64{float64(ogWidth), float64(ogHeight)},
					[]float64{maxWidth, maxHeight},
				)
				sizeX[i] = int16(width)
				sizeY[i] = int16(height)

				// Create new boundaries for frames
				mw := imagick.NewMagickWand() // Get magick wand & read the original image
				if err = mw.SetResourceLimit(imagick.RESOURCE_MEMORY, 500); err != nil {
					log.WithError(err).Error("SetResourceLimit")
				}
				if err := mw.ReadImage(ogFilePath); err != nil {
					return restutil.ErrBadRequest().Send(c, fmt.Sprintf("Input File Not Readable: %s", err))
				}

				// Merge all frames with coalesce
				aw := mw.CoalesceImages()
				if err = aw.SetResourceLimit(imagick.RESOURCE_MEMORY, 500); err != nil {
					log.WithError(err).Error("SetResourceLimit")
				}
				mw.Destroy()
				defer aw.Destroy()

				// Set delays
				mw = imagick.NewMagickWand()
				if err = mw.SetResourceLimit(imagick.RESOURCE_MEMORY, 500); err != nil {
					log.WithError(err).Error("SetResourceLimit")
				}
				defer mw.Destroy()

				// Add each frame to our animated image
				mw.ResetIterator()
				for ind := 0; ind < int(aw.GetNumberImages()); ind++ {
					aw.SetIteratorIndex(ind)
					img := aw.GetImage()

					if err = img.ResizeImage(uint(width), uint(height), imagick.FILTER_LANCZOS); err != nil {
						log.WithError(err).Errorf("ResizeImage i=%v", ind)
						continue
					}
					if err = mw.AddImage(img); err != nil {
						log.WithError(err).Errorf("AddImage i=%v", ind)
					}
					img.Destroy()
				}

				// Done - convert to WEBP
				q, _ := strconv.Atoi(quality)
				if err = mw.SetImageCompressionQuality(uint(q)); err != nil {
					log.WithError(err).Error("SetImageCompressionQuality")
				}
				if err = mw.SetImageFormat("webp"); err != nil {
					log.WithError(err).Error("SetImageFormat")
				}

				// Write to file
				err = mw.WriteImages(outFile, true)
				if err != nil {
					log.WithError(err).Error("cmd")
					return restutil.ErrInternalServer().Send(c)
				}
			}

			wg := &sync.WaitGroup{}
			wg.Add(len(files))

			emote = &datastructure.Emote{
				Name:             emoteName,
				Mime:             mime,
				Status:           datastructure.EmoteStatusProcessing,
				Tags:             utils.Ternary(emoteTags != nil, emoteTags, []string{}).([]string),
				Visibility:       datastructure.EmoteVisibilityPrivate | datastructure.EmoteVisibilityUnlisted,
				OwnerID:          *channelID,
				LastModifiedDate: time.Now(),
				Width:            sizeX,
				Height:           sizeY,
			}
			res, err := mongo.Collection(mongo.CollectionNameEmotes).InsertOne(c.Context(), emote)

			if err != nil {
				log.WithError(err).Error("mongo")
				return restutil.ErrInternalServer().Send(c)
			}

			_id, ok := res.InsertedID.(primitive.ObjectID)
			if !ok {
				log.WithField("resp", res.InsertedID).Error("bad resp from mongo")
				_, err := mongo.Collection(mongo.CollectionNameEmotes).DeleteOne(c.Context(), bson.M{
					"_id": res.InsertedID,
				})
				if err != nil {
					log.WithError(err).Error("mongo")
				}
				return restutil.ErrInternalServer().Send(c)
			}

			emote.ID = _id
			errored := false

			for _, path := range files {
				go func(path []string) {
					defer wg.Done()
					data, err := os.ReadFile(path[0] + ".webp")
					if err != nil {
						log.WithError(err).Error("read")
						errored = true
						return
					}

					if err := aws.UploadFile(configure.Config.GetString("aws_cdn_bucket"), fmt.Sprintf("emote/%s/%s", _id.Hex(), path[1]), data, &mime); err != nil {
						log.WithError(err).Error("aws")
						errored = true
					}
				}(path)
			}

			wg.Wait()

			if errored {
				_, err := mongo.Collection(mongo.CollectionNameEmotes).DeleteOne(c.Context(), bson.M{
					"_id": _id,
				})
				if err != nil {
					log.WithError(err).WithField("id", id).Error("mongo")
				}
				return restutil.ErrInternalServer().Send(c)
			}

			_, err = mongo.Collection(mongo.CollectionNameEmotes).UpdateOne(c.Context(), bson.M{
				"_id": _id,
			}, bson.M{
				"$set": bson.M{
					"status": datastructure.EmoteStatusLive,
				},
			})
			if err != nil {
				log.WithError(err).WithField("id", id).Error("mongo")
			}

			_, err = mongo.Collection(mongo.CollectionNameAudit).InsertOne(c.Context(), &datastructure.AuditLog{
				Type: datastructure.AuditLogTypeEmoteCreate,
				Changes: []*datastructure.AuditLogChange{
					{Key: "name", OldValue: nil, NewValue: emoteName},
					{Key: "tags", OldValue: nil, NewValue: []string{}},
					{Key: "owner", OldValue: nil, NewValue: usr.ID},
					{Key: "visibility", OldValue: nil, NewValue: datastructure.EmoteVisibilityPrivate},
					{Key: "mime", OldValue: nil, NewValue: mime},
					{Key: "status", OldValue: nil, NewValue: datastructure.EmoteStatusProcessing},
				},
				Target:    &datastructure.Target{ID: &_id, Type: "emotes"},
				CreatedBy: usr.ID,
			})
			if err != nil {
				log.WithError(err).Error("mongo")
			}

			go discord.SendEmoteCreate(*emote, *usr)
			return c.SendString(fmt.Sprintf(`{"id":"%v"}`, emote.ID.Hex()))
		})
}

func getGifDimensions(gif *gif.GIF) (x, y int) {
	var leastX int
	var leastY int
	var mostX int
	var mostY int

	for _, img := range gif.Image {
		if img.Rect.Min.X < leastX {
			leastX = img.Rect.Min.X
		}
		if img.Rect.Min.Y < leastY {
			leastY = img.Rect.Min.Y
		}
		if img.Rect.Max.X > mostX {
			mostX = img.Rect.Max.X
		}
		if img.Rect.Max.Y > mostY {
			mostY = img.Rect.Max.Y
		}
	}

	return mostX - leastX, mostY - leastY
}
