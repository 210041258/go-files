const userController = require("../controllers/user.controller");
const auth = require("../middleware/auth.middleware");

// All routes require JWT
router.get("/", auth, userController.getUsers);
router.get("/:id", auth, userController.getUserById);
router.post("/", auth, userController.createUser);
router.put("/:id", auth, userController.updateUser);
router.delete("/:id", auth, userController.deleteUser);


module.exports = router;
