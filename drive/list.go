package drive

import (
	"fmt"
	"golang.org/x/net/context"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/googleapi"
	"io"
	"text/tabwriter"
	"os"
	"encoding/csv"	
)

type ListFilesArgs struct {
	Out         io.Writer
	MaxFiles    int64
	NameWidth   int64
	Query       string
	SortOrder   string
	SkipHeader  bool
	SizeInBytes bool
	AbsPath     bool
	UseCsv		bool
	UseExtended	bool
}

func (self *Drive) List(args ListFilesArgs) (err error) {
	listArgs := listAllFilesArgs{
		query:     args.Query,
		fields:    []googleapi.Field{"nextPageToken", "files(id, name, md5Checksum, mimeType, size, createdTime, parents, headRevisionId)"},
		sortOrder: args.SortOrder,
		maxFiles:  args.MaxFiles,
	}
	
	files, err := self.listAllFiles(listArgs)
	if err != nil {
		return fmt.Errorf("Failed to list files: %s", err)
	}

	pathfinder := self.newPathfinder()

	if args.AbsPath {
		// Replace name with absolute path
		for _, f := range files {
			f.Name, err = pathfinder.absPath(f)
			if err != nil {
				return err
			}
		}
	}

	printArgs := PrintFileListArgs{
			Out:         args.Out,
			Files:       files,
			NameWidth:   int(args.NameWidth),
			SkipHeader:  args.SkipHeader,
			SizeInBytes: args.SizeInBytes,
			Delimiter:   '|',
			UseExtended: args.UseExtended,
		}
	
	if args.UseCsv {
		PrintFileList(printArgs)
	} else {
		PrintTabbedFileList(printArgs)
	}

	return
}

type listAllFilesArgs struct {
	query     string
	fields    []googleapi.Field
	sortOrder string
	maxFiles  int64
}

func (self *Drive) listAllFiles(args listAllFilesArgs) ([]*drive.File, error) {
	var files []*drive.File

	var pageSize int64
	if args.maxFiles > 0 && args.maxFiles < 1000 {
		pageSize = args.maxFiles
	} else {
		pageSize = 1000
	}

	controlledStop := fmt.Errorf("Controlled stop")

	err := self.service.Files.List().Q(args.query).Fields(args.fields...).OrderBy(args.sortOrder).PageSize(pageSize).Pages(context.TODO(), func(fl *drive.FileList) error {
		files = append(files, fl.Files...)

		// Stop when we have all the files we need
		if args.maxFiles > 0 && len(files) >= int(args.maxFiles) {
			return controlledStop
		}

		return nil
	})

	if err != nil && err != controlledStop {
		return nil, err
	}

	if args.maxFiles > 0 {
		n := min(len(files), int(args.maxFiles))
		return files[:n], nil
	}

	return files, nil
}

type PrintFileListArgs struct {
	Out         io.Writer
	Files       []*drive.File
	NameWidth   int
	SkipHeader  bool
	SizeInBytes bool
	Delimiter	rune
	UseExtended bool
}

func PrintFileList(args PrintFileListArgs) {
	w := csv.NewWriter(os.Stdout)
	w.Comma = args.Delimiter

	if !args.SkipHeader {
	
		headers := []string{"Id", "Name", "Type", "Size", "Created"}
	
		if args.UseExtended {
			headers = append(headers, []string{"Checksum", "HeadRevisionId"}...)
		}
		
		w.Write(headers)
	}

	var records [][]string
	
	for _, f := range args.Files {
		
		record := []string{
			f.Id,
			truncateString(f.Name, args.NameWidth),
			filetype(f),
			formatSize(f.Size, args.SizeInBytes),
			formatDatetime(f.CreatedTime),
		}
		
		if args.UseExtended {
			record = append(record, []string{f.Md5Checksum, f.HeadRevisionId}...)
		}
		
		records = append(records, record)
	}
	
	w.WriteAll(records)
	w.Flush()
}

func PrintTabbedFileList(args PrintFileListArgs) {
	w := new(tabwriter.Writer)
	w.Init(args.Out, 0, 0, 3, ' ', 0)

	if !args.SkipHeader {
		fmt.Fprintln(w, "Id\tName\tType\tSize\tCreated")
	}

	for _, f := range args.Files {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			f.Id,
			truncateString(f.Name, args.NameWidth),
			filetype(f),
			formatSize(f.Size, args.SizeInBytes),
			formatDatetime(f.CreatedTime),
		)
	}

	w.Flush()
}

func filetype(f *drive.File) string {
	if isDir(f) {
		return "dir"
	} else if isBinary(f) {
		return "bin"
	}
	return "doc"
}
