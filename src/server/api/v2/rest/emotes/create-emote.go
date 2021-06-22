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
const MAX_FILE_SIZE = 5000000

func CreateEmoteRoute(router fiber.Router) {

	rl := configure.Config.GetIntSlice("limits.route.emote-create")
	router.Post(
		"/",
		middleware.UserAuthMiddleware(true),
		middleware.RateLimitMiddleware("emote-create", int32(rl[0]), time.Millisecond*time.Duration(rl[1])),
		middleware.AuditRoute(func(c *fiber.Ctx) (int, []byte, *datastructure.AuditLog) {
			c.Set("Content-Type", "application/json")
			usr, ok := c.Locals("user").(*datastructure.User)
			if !ok {
				return 500, errInternalServer, nil
			}
			if !usr.HasPermission(datastructure.RolePermissionEmoteCreate) {
				return 403, utils.S2B(fmt.Sprintf(errAccessDenied, "You don't have permission to do that.")), nil
			}

			req := c.Request()
			fctx := c.Context()
			if !req.IsBodyStream() {
				return 400, utils.S2B(fmt.Sprintf(errInvalidRequest, "You did not provide an upload stream.")), nil
			}

			// Get file stream
			file := fctx.RequestBodyStream()
			mr := multipart.NewReader(file, utils.B2S(req.Header.MultipartFormBoundary()))
			var emote *datastructure.Emote
			var emoteName string              // The name of the emote
			var channelID *primitive.ObjectID // The channel creating this emote
			var contentType string
			var ext string
			id, _ := uuid.NewRandom()

			// The temp directory where the emote will be created
			fileDir := fmt.Sprintf("%s/%s", configure.Config.GetString("temp_file_store"), id.String())
			if err := os.MkdirAll(fileDir, 0777); err != nil {
				log.WithError(err).Error("mkdir")
				return 500, errInternalServer, nil
			}
			ogFilePath := fmt.Sprintf("%v/og", fileDir) // The original file's path in temp

			// Remove temp dir once this function completes
			defer os.RemoveAll(fileDir)

			// Get form data parts
			channelID = &usr.ID // Default channel ID to the uploader
			for i := 0; i < 3; i++ {
				part, err := mr.NextPart()
				if err != nil {
					continue
				}

				if part.FormName() == "name" {
					buf := make([]byte, 32)
					n, err := part.Read(buf)
					if err != nil && err != io.EOF {
						return 400, utils.S2B(fmt.Sprintf(errInvalidRequest, "We couldn't read the name.")), nil
					}

					if !validation.ValidateEmoteName(buf[:n]) {
						return 400, utils.S2B(fmt.Sprintf(errInvalidRequest, "Invalid Emote Name")), nil
					}
					emoteName = utils.B2S(buf[:n])
				} else if part.FormName() == "channel" {
					buf := make([]byte, 64)
					n, err := part.Read(buf)
					if err != nil && err != io.EOF {
						return 400, utils.S2B(fmt.Sprintf(errInvalidRequest, "We couldn't read the channel ID.")), nil
					}
					id, err := primitive.ObjectIDFromHex(utils.B2S(buf[:n]))
					if err != nil {
						return 400, utils.S2B(fmt.Sprintf(errInvalidRequest, "The channel ID is not valid.")), nil
					}
					channelID = &id
				} else if part.FormName() == "emote" {
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
						return 400, utils.S2B(fmt.Sprintf(errInvalidRequest, "The file content type is not supported. It must be one of jpg, png or gif")), nil
					}

					osFile, err := os.Create(ogFilePath)
					if err != nil {
						log.WithError(err).Error("file")
						return 500, errInternalServer, nil
					}

					byteSize := 0
					for {
						n, err := part.Read(data)
						byteSize += n
						if byteSize >= MAX_FILE_SIZE {
							return 400, utils.S2B(fmt.Sprintf(errInvalidRequest, "The file is too large.")), nil
						}

						if err != nil && err != io.EOF {
							log.WithError(err).Error("read")
							return 400, utils.S2B(fmt.Sprintf(errInvalidRequest, "We failed to read the file.")), nil
						}
						_, err2 := osFile.Write(data[:n])
						if err2 != nil {
							osFile.Close()
							log.WithError(err).Error("write")
							return 500, errInternalServer, nil
						}
						if err == io.EOF {
							break
						}
					}
				}
			}

			if emoteName == "" || channelID == nil {
				return 400, utils.S2B(fmt.Sprintf(errInvalidRequest, "The fields were not provided.")), nil
			}

			if !usr.HasPermission(datastructure.RolePermissionManageUsers) {
				if channelID.Hex() != usr.ID.Hex() {
					if err := mongo.Database.Collection("users").FindOne(c.Context(), bson.M{
						"_id":     channelID,
						"editors": usr.ID,
					}).Err(); err != nil {
						if err == mongo.ErrNoDocuments {
							return 403, utils.S2B(fmt.Sprintf(errAccessDenied, "You don't have permission to do that.")), nil
						}
						log.WithError(err).Error("mongo")
						return 500, errInternalServer, nil
					}
				}
			}

			// Get uploaded image file into an image.Image
			ogFile, err := os.Open(ogFilePath)
			if err != nil {
				log.WithError(err).Error("could not open original file")
				return 500, errInternalServer, nil
			}
			ogHeight := 0
			ogWidth := 0
			switch ext {
			case "jpg":
				img, err := jpeg.Decode(ogFile)
				if err != nil {
					log.WithError(err).Error("could not decode jpeg")
					return 500, errInternalServer, nil
				}
				ogWidth = img.Bounds().Dx()
				ogHeight = img.Bounds().Dy()
			case "png":
				img, err := png.Decode(ogFile)
				if err != nil {
					log.WithError(err).Error("could not decode png")
					return 500, errInternalServer, nil
				}
				ogWidth = img.Bounds().Dx()
				ogHeight = img.Bounds().Dy()
			case "gif":
				g, err := gif.DecodeAll(ogFile)
				if err != nil {
					log.WithError(err).Error("could not decode gif")
					return 500, errInternalServer, nil
				}

				// Set a cap on how many frames are allowed
				if len(g.Image) > MAX_FRAME_COUNT {
					return 400, utils.S2B(fmt.Sprintf(errInvalidRequest, fmt.Sprintf("Your GIF exceeds the maximum amount of frames permitted. (%v)", MAX_FRAME_COUNT))), nil
				}

				ogWidth, ogHeight = getGifDimensions(g)
			case "webp":
				wand := imagick.NewMagickWand()
				if err := wand.ReadImageFile(ogFile); err == nil {
					ogWidth = int(wand.GetImageWidth())
					ogHeight = int(wand.GetImageHeight())
				} else {
					log.WithError(err).Error("could not decode webp")
					return 500, utils.S2B(fmt.Sprintf(errInvalidRequest, err.Error())), nil
				}

				wand.Destroy()
			default:
				return 500, utils.S2B(fmt.Sprintf(errInvalidRequest, "Unsupported File Format")), nil
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
				if err := mw.ReadImage(ogFilePath); err != nil {
					return 500, utils.S2B(fmt.Sprintf(errInvalidRequest, "Couldn't read original image: %s", err)), nil
				}
				delay := mw.GetImageDelay()

				// Merge all frames with coalesce
				aw := mw.CoalesceImages()
				mw.Destroy()
				defer aw.Destroy()

				// Set delays
				mw = imagick.NewMagickWand()
				mw.SetImageDelay(delay)
				defer mw.Destroy()

				// Add each frame to our animated image
				for ind := 0; ind < int(aw.GetNumberImages()); ind++ {
					aw.SetIteratorIndex(ind)
					img := aw.GetImage()
					img.ResizeImage(uint(width), uint(height), imagick.FILTER_LANCZOS)
					mw.AddImage(img)
					img.Destroy()
				}

				// Done - convert to WEBP
				mw.ResetIterator()
				q, _ := strconv.Atoi(quality)
				mw.SetImageCompressionQuality(uint(q))
				mw.SetImageFormat("webp")

				// Write to file
				err = mw.WriteImages(outFile, true)
				if err != nil {
					log.WithError(err).Error("cmd")
					return 500, errInternalServer, nil
				}
			}

			wg := &sync.WaitGroup{}
			wg.Add(len(files))

			emote = &datastructure.Emote{
				Name:             emoteName,
				Mime:             mime,
				Status:           datastructure.EmoteStatusProcessing,
				Tags:             []string{},
				Visibility:       datastructure.EmoteVisibilityPrivate | datastructure.EmoteVisibilityUnlisted,
				OwnerID:          *channelID,
				LastModifiedDate: time.Now(),
				Width:            sizeX,
				Height:           sizeY,
			}
			res, err := mongo.Database.Collection("emotes").InsertOne(c.Context(), emote)

			if err != nil {
				log.WithError(err).Error("mongo")
				return 500, errInternalServer, nil
			}

			_id, ok := res.InsertedID.(primitive.ObjectID)
			if !ok {
				log.WithField("resp", res.InsertedID).Error("bad resp from mongo")
				_, err := mongo.Database.Collection("emotes").DeleteOne(c.Context(), bson.M{
					"_id": res.InsertedID,
				})
				if err != nil {
					log.WithError(err).Error("mongo")
				}
				return 500, errInternalServer, nil
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
				_, err := mongo.Database.Collection("emotes").DeleteOne(c.Context(), bson.M{
					"_id": _id,
				})
				if err != nil {
					log.WithError(err).WithField("id", id).Error("mongo")
				}
				return 500, errInternalServer, nil
			}

			_, err = mongo.Database.Collection("emotes").UpdateOne(c.Context(), bson.M{
				"_id": _id,
			}, bson.M{
				"$set": bson.M{
					"status": datastructure.EmoteStatusLive,
				},
			})
			if err != nil {
				log.WithError(err).WithField("id", id).Error("mongo")
			}

			go discord.SendEmoteCreate(*emote, *usr)
			return 201, utils.S2B(fmt.Sprintf(`{"status":201,"id":"%s"}`, _id.Hex())), &datastructure.AuditLog{
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
			}
		}))
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
