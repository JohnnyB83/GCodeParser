package main
 
import (
    "bufio"
    "fmt"
    "os"
    "strings"
    "strconv"
    "math"
    "encoding/json"
)

type PrintData struct {
    ThumbnailImage string `json:thumbnailImage`
    EstimatedPrintingTime string `json:estimatedPrintingTime`
    FilamentAmountG string `json:filamentAmountG`
    FilamentAmountMM string `json:filamentAmountMM`
    FilamentAmountCM3 string `json:filamentAmountCM3`
    FilamentCost string `json:filamentCost`
    PosArr []float32 `json:posArr`
    IndexArr []int `json:indexArr`
    ColorArr []float32 `json:colorArr`
}

var plasticColor = make([]float32, 3);

var lastX float32 = 0.0;
var lastY float32 = 0.0;
var lastZ float32 = 0.0;

var isExtrudingGlobal bool = false;

var gCodePoints []float32;
var gCodeColors []float32;
var gCodeIndex []int;

var currentSegmentIndex []int;
var currentIndex int = 0;
var lastNonExtrusionIndex int = 0;

func doesMovementContainNewCoords(gCodeArr []string) bool {
    for i := 0; i < len(gCodeArr); i++ {
        if (string(gCodeArr[i][0]) == "X") {
            return true;
        } else if (string(gCodeArr[i][0]) == "Y") {
            return true;
        } else if (string(gCodeArr[i][0]) == "Z") {
            return true;
        }
    }
    return false;
}

func parseGCode(gCodeString string) {
    gCodeArr := strings.Fields(gCodeString);
    isExtruding := false;

    // Determine what color the line should be
    if (strings.Contains(gCodeString, ";TYPE:")) {
        specialInstruciton := strings.Split(gCodeString, ":");
        switch specialInstruciton[1] {
            case "Perimeter":
                plasticColor[0] = 1.0;
                plasticColor[1] = 1.0;
                plasticColor[2] = 0.0;
            case "External perimter":
                plasticColor[0] = 0.96;
                plasticColor[1] = 0.505;
                plasticColor[2] = 0.019;
            case "Internal infill":
                plasticColor[0] = 0.509;
                plasticColor[1] = 0.188;
                plasticColor[2] = 0.016;
            case "Solid infill":
                plasticColor[0] = 0.862;
                plasticColor[1] = 0.058;
                plasticColor[2] = 1.0;
            case "Top solid infill":
                plasticColor[0] = 0.968;
                plasticColor[1] = 0.176;
                plasticColor[2] = 0.255;
            case "Bridge infill":
                plasticColor[0] = 0.094;
                plasticColor[1] = 1.0;
                plasticColor[2] = 0.921;
            case "Skirt/Brim":
                plasticColor[0] = 0.015;
                plasticColor[1] = 0.721;
                plasticColor[2] = 0.392;
            case "Custom":
                plasticColor[0] = 0.250;
                plasticColor[1] = 0.960;
                plasticColor[2] = 0.627;
            default:
                plasticColor[0] = 1.0;
                plasticColor[1] = 1.0;
                plasticColor[2] = 1.0;
            }
    }

    if ((gCodeArr[0] == "G0" || gCodeArr[0] == "G1") && doesMovementContainNewCoords(gCodeArr)) {
        var currentX float32 = -1.0;
        var currentY float32 = -1.0;
        var currentZ float32 = -1.0;

        for i := 0; i < len(gCodeArr); i++ {
            if (string(gCodeArr[i][0]) == "X") {
                xPos, _ := strconv.ParseFloat(gCodeArr[i][1:], 32);
                currentX = float32(math.Floor(xPos * 100) / 100);
            } else if (string(gCodeArr[i][0]) == "Y") {
                yPos, _ := strconv.ParseFloat(gCodeArr[i][1:], 32);
                currentY = float32(math.Floor(yPos * 100) / 100);
            } else if (string(gCodeArr[i][0]) == "Z") {
                zPos, _ := strconv.ParseFloat(gCodeArr[i][1:], 32);
                currentZ = float32(math.Floor(zPos * 100) / 100);
            } else if (string(gCodeArr[i][0]) == "E") {
                eVal, _ := strconv.ParseFloat(gCodeArr[i][1:], 32);
                if (eVal > 0) {
                    isExtruding = true;
                }
            }
        }

        if (currentX > 0) {
            lastX = currentX;
        }
        if (currentY > 0) {
            lastY = currentY;
        }
        if (currentZ > 0) {
            lastZ = currentZ;
        }

        if (isExtruding && isExtrudingGlobal) {
            // Continuation of line
            gCodePoints = append(gCodePoints, lastX, lastY, lastZ);
            gCodeColors = append(gCodeColors, plasticColor[0], plasticColor[1], plasticColor[2]);

            currentSegmentIndex = append(currentSegmentIndex, currentIndex - 1, currentIndex);
        } else if (!isExtruding && isExtrudingGlobal) {
            // Line has ended
            gCodePoints = append(gCodePoints, lastX, lastY, lastZ);
            gCodeColors = append(gCodeColors, plasticColor[0], plasticColor[1], plasticColor[2]);

            gCodeIndex = append(gCodeIndex, currentSegmentIndex...)
            currentSegmentIndex = nil;
            isExtrudingGlobal = false;
        } else if (isExtruding && !isExtrudingGlobal) {
            // New line
            gCodePoints = append(gCodePoints, lastX, lastY, lastZ);
            gCodeColors = append(gCodeColors, plasticColor[0], plasticColor[1], plasticColor[2]);

            currentSegmentIndex = append(currentSegmentIndex, lastNonExtrusionIndex, currentIndex);
            isExtrudingGlobal = true;

        } else {
            // Not extruding
            gCodePoints = append(gCodePoints, lastX, lastY, lastZ);
            gCodeColors = append(gCodeColors, plasticColor[0], plasticColor[1], plasticColor[2]);

            lastNonExtrusionIndex = currentIndex;

        }
        currentIndex++;
    }
}

// Function to parse through GCode and create the JSON file needed for the frontend
// To show the rendered drawing / info about the print
func main() {
    filePath := os.Args[1]
    readFile, err := os.Open(filePath)
  
    if err != nil {
        fmt.Println(err)
    }
    fileScanner := bufio.NewScanner(readFile)
    fileScanner.Split(bufio.ScanLines)

    f, _ := os.Create("./test.json")
    w := bufio.NewWriter(f)

    var recordImage = false;
    var recordGcode = false;

    var thumbnailImage string;
    var estimatedPrintingTime string;
    var filamentAmountG string;
    var filamentAmountMM string;
    var filamentAmountCM3 string;
    var filamentCost string;
  
    for fileScanner.Scan() {
        var currentLine = fileScanner.Text()
        if (len(currentLine) > 0) {
            if (recordImage) {
                if (strings.Contains(currentLine, "thumbnail end")) {
                    recordImage = false;
                    recordGcode = true;
                } else {
                    thumbnailImage += strings.Trim(currentLine[1:], " ");
                }
            }
            if (recordGcode) {
                parseGCode(currentLine);
            }
            if (strings.Contains(currentLine, "thumbnail begin")) {
                recordImage = true;
            }
            if (strings.Contains(currentLine, "Filament-specific end gcode")) {
                recordGcode = false;
            }

            if (strings.Contains(currentLine, "estimated printing time (normal mode)")) {
                time := strings.Index(currentLine, "=");
                estimatedPrintingTime = strings.Trim(currentLine[time+1:], " ");
            }

            if (strings.Contains(currentLine, "filament used [g]")) {
                amount := strings.Index(currentLine, "=");
                filamentAmountG = strings.Trim(currentLine[amount+1:], " ");
            }

            if (strings.Contains(currentLine, "filament used [mm]")) {
                amount := strings.Index(currentLine, "=");
                filamentAmountMM = strings.Trim(currentLine[amount+1:], " ");
            }

            if (strings.Contains(currentLine, "filament used [cm3]")) {
                amount := strings.Index(currentLine, "=");
                filamentAmountCM3 = strings.Trim(currentLine[amount+1:], " ");
            }

            if (strings.Contains(currentLine, "total filament cost")) {
                amount := strings.Index(currentLine, "=");
                filamentCost = strings.Trim(currentLine[amount+1:], " ");
            }
        }
    }
    readFile.Close()

    data := PrintData{thumbnailImage, estimatedPrintingTime, filamentAmountG, filamentAmountMM, filamentAmountCM3, filamentCost, gCodePoints, gCodeIndex, gCodeColors};
    b,err := json.Marshal(data);
    if err != nil {
		fmt.Printf("could not marshal json: %s\n", err)
		return
	}
    w.Write(b)
}
