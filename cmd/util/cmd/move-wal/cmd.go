package move_wal

import (
	"os"

	prometheusWAL "github.com/onflow/wal/wal"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

var (
	flagFrom     int
	flagTo       int
	flagMoveFrom string
	flagMoveTo   string
	flagSend     bool
)

var Cmd = &cobra.Command{
	Use:   "move-wal",
	Short: "Move WAL segment files from one directory to another",
	Run:   run,
}

func init() {
	Cmd.Flags().IntVar(&flagFrom, "from", 0, "from segment number (inclusive)")
	_ = Cmd.MarkFlagRequired("from")

	Cmd.Flags().IntVar(&flagTo, "to", -1, "to segment number (inclusive, optional - if not set, moves all files from --from)")

	Cmd.Flags().StringVar(&flagMoveFrom, "move-from", "",
		"source directory containing WAL files")
	_ = Cmd.MarkFlagRequired("move-from")

	Cmd.Flags().StringVar(&flagMoveTo, "move-to", "",
		"destination directory to move WAL files to")
	_ = Cmd.MarkFlagRequired("move-to")

	Cmd.Flags().BoolVar(&flagSend, "send", false,
		"actually perform the move operation (default: false, preview only)")
}

func run(*cobra.Command, []string) {
	// Validate directories
	if flagMoveFrom == flagMoveTo {
		log.Fatal().Msg("--move-from and --move-to directories cannot be the same")
	}

	// Check source directory exists
	srcInfo, err := os.Stat(flagMoveFrom)
	if err != nil {
		log.Fatal().Err(err).Str("dir", flagMoveFrom).Msg("source directory does not exist or cannot be accessed")
	}
	if !srcInfo.IsDir() {
		log.Fatal().Str("dir", flagMoveFrom).Msg("--move-from must be a directory")
	}

	// Check/create destination directory (only if --send is true)
	if flagSend {
		dstInfo, err := os.Stat(flagMoveTo)
		if err != nil {
			if os.IsNotExist(err) {
				log.Info().Str("dir", flagMoveTo).Msg("creating destination directory")
				err = os.MkdirAll(flagMoveTo, 0755)
				if err != nil {
					log.Fatal().Err(err).Str("dir", flagMoveTo).Msg("cannot create destination directory")
				}
			} else {
				log.Fatal().Err(err).Str("dir", flagMoveTo).Msg("cannot access destination directory")
			}
		} else if !dstInfo.IsDir() {
			log.Fatal().Str("dir", flagMoveTo).Msg("--move-to must be a directory")
		}
	} else {
		// In preview mode, just check if destination path is valid (but don't create it)
		log.Info().Bool("send", flagSend).Msg("preview mode: --send flag not set, will only log what would be moved")
	}

	// Get available segments in source directory
	fromSegment, toSegment, err := prometheusWAL.Segments(flagMoveFrom)
	if err != nil {
		log.Fatal().Err(err).Str("dir", flagMoveFrom).Msg("cannot get segments from source directory")
	}

	if fromSegment < 0 {
		log.Fatal().Str("dir", flagMoveFrom).Msg("no WAL segments found in source directory")
	}

	// Determine the actual range to move
	actualFrom := flagFrom
	var actualTo int

	// If --to is not set (still -1), use the last available segment
	if flagTo == -1 {
		actualTo = toSegment
		log.Info().Int("from", actualFrom).Int("to", actualTo).Msg("--to not specified, will move all files from --from to last available segment")
	} else {
		actualTo = flagTo
	}

	// Validate and adjust --from if needed
	if actualFrom < fromSegment {
		log.Info().Int("requested", actualFrom).Int("available", fromSegment).Msg("adjusting --from to first available segment")
		actualFrom = fromSegment
	}

	// Validate and adjust --to if needed
	if actualTo > toSegment {
		log.Info().Int("requested", actualTo).Int("available", toSegment).Msg("adjusting --to to last available segment")
		actualTo = toSegment
	}

	if actualFrom > actualTo {
		log.Fatal().
			Int("from", actualFrom).
			Int("to", actualTo).
			Msg("invalid segment range: from is greater than to")
	}

	if flagSend {
		log.Info().
			Int("from", actualFrom).
			Int("to", actualTo).
			Str("source", flagMoveFrom).
			Str("destination", flagMoveTo).
			Msg("moving WAL segments")
	} else {
		log.Info().
			Int("from", actualFrom).
			Int("to", actualTo).
			Str("source", flagMoveFrom).
			Str("destination", flagMoveTo).
			Msg("preview: would move WAL segments (use --send=true to actually move)")
	}

	// Move each segment file in the range
	movedCount := 0
	for i := actualFrom; i <= actualTo; i++ {
		srcFile := prometheusWAL.SegmentName(flagMoveFrom, i)
		dstFile := prometheusWAL.SegmentName(flagMoveTo, i)

		// Check if source file exists
		_, err := os.Stat(srcFile)
		if err != nil {
			if os.IsNotExist(err) {
				log.Debug().Int("segment", i).Str("file", srcFile).Msg("segment file does not exist, skipping")
				continue
			}
			log.Fatal().Err(err).Int("segment", i).Str("file", srcFile).Msg("cannot stat source segment file")
		}

		// Check if destination file already exists
		_, err = os.Stat(dstFile)
		if err == nil {
			if flagSend {
				log.Warn().Int("segment", i).Str("file", dstFile).Msg("destination file already exists, skipping")
			} else {
				log.Warn().Int("segment", i).Str("file", dstFile).Msg("preview: destination file already exists, would skip")
			}
			continue
		}

		if flagSend {
			// Move the file
			log.Info().Int("segment", i).Str("src", srcFile).Str("dst", dstFile).Msg("moving segment file")
			err = os.Rename(srcFile, dstFile)
			if err != nil {
				log.Fatal().Err(err).
					Int("segment", i).
					Str("src", srcFile).
					Str("dst", dstFile).
					Msg("cannot move segment file")
			}
		} else {
			// Preview mode: just log what would be moved
			log.Info().Int("segment", i).Str("src", srcFile).Str("dst", dstFile).Msg("preview: would move segment file")
		}

		movedCount++
	}

	if flagSend {
		log.Info().
			Int("moved_count", movedCount).
			Int("from", actualFrom).
			Int("to", actualTo).
			Msg("finished moving WAL segments")
	} else {
		log.Info().
			Int("would_move_count", movedCount).
			Int("from", actualFrom).
			Int("to", actualTo).
			Msg("preview complete: use --send=true to actually move the files")
	}
}
